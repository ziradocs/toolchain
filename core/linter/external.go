package linter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// themeVariablesEnvVar es el nombre de la variable de entorno por la que un
// rulepack externo (subproceso) recibe el mapa de variables CSS del tema
// activo, como JSON. Los rulepacks reciben el AST por stdin (ver
// runExternalRulepack) y ese envelope NO cambia — agregar un campo ahí
// rompería cualquier rulepack existente que hoy parsea un AST desnudo. El
// canal de tema es aditivo y por su naturaleza ignorable: un rulepack que
// no lo lee simplemente no ve la variable de entorno.
const themeVariablesEnvVar = "ZIRADOCS_THEME_VARIABLES"

// filterOutEnvVar devuelve env sin ninguna entrada cuyo nombre de variable
// coincida con key, comparando SIN distinguir mayúsculas/minúsculas
// (strings.EqualFold) — Windows (una plataforma publicada por este
// toolchain, ver .goreleaser.yaml) trata los nombres de variable de entorno
// sin distinguir mayúsculas, así que un valor heredado como
// "ziradocs_theme_variables=..." debe filtrarse igual que la forma exacta;
// una comparación case-sensitive lo dejaría pasar y llegaría al rulepack
// aunque este seam nunca se haya usado en esta corrida.
func filterOutEnvVar(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if name, _, found := strings.Cut(e, "="); found && strings.EqualFold(name, key) {
			continue
		}
		out = append(out, e)
	}
	return out
}

type externalManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Prefix  string `json:"prefix"`
}

type externalReport struct {
	ReportVersion string            `json:"reportVersion"`
	Manifest      externalManifest  `json:"manifest"`
	Findings      []externalFinding `json:"findings"`
	// Descriptors se decodifica aparte (json.RawMessage), no inline: un
	// descriptor malformado NO debe hacer fallar el Unmarshal del reporte
	// completo y descartar los findings reales del pack (pérdida silenciosa
	// de diagnósticos = falso "limpio").
	Descriptors json.RawMessage `json:"descriptors,omitempty"`
}

type externalFinding struct {
	diagnostics.Diagnostic
}

func runExternalRulepack(doc *ast.AST, binaryPath string, timeout time.Duration, themeVariables map[string]string, themeVariablesSet bool) ([]diagnostics.Diagnostic, []RuleDescriptor, error) {
	// Raw AST: json.Marshal directly on *ast.AST.
	input, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing AST for rulepack: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Partimos SIEMPRE de os.Environ() sin ninguna ZIRADOCS_THEME_VARIABLES
	// heredada: si esta corrida no participa del seam #30, un rulepack no
	// debe ver una variable que quedó de una invocación previa/no
	// relacionada en el mismo proceso padre (p. ej. un wrapper de CI, o un
	// embedder de vida larga) y atribuirle esos colores al documento
	// actual. Solo si WithThemeVariables se llamó (aunque sea con un mapa
	// vacío) se agrega la variable de vuelta, con su valor de ESTA corrida.
	env := filterOutEnvVar(os.Environ(), themeVariablesEnvVar)
	if themeVariablesSet {
		// nil serializa a JSON "null", no "{}" — un rulepack que decodifica
		// a un objeto (o hace unmarshal directo a map[string]string y
		// verifica que no sea nil) no debe recibir un valor que rompe la
		// forma de objeto documentada por variar según si el tema resolvió
		// con o sin variables.
		vars := themeVariables
		if vars == nil {
			vars = map[string]string{}
		}
		themeJSON, err := json.Marshal(vars)
		if err != nil {
			return nil, nil, fmt.Errorf("serializing theme variables for rulepack: %w", err)
		}
		env = append(env, themeVariablesEnvVar+"="+string(themeJSON))
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, nil, fmt.Errorf("timed out after %s", timeout)
		}
		return nil, nil, fmt.Errorf("exited with error: %w (stderr: %s)", err, stderr.String())
	}

	var report externalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return nil, nil, fmt.Errorf("decoding rulepack output: %w", err)
	}

	if report.ReportVersion == "" {
		return nil, nil, fmt.Errorf("missing reportVersion in rulepack output")
	}
	if report.Manifest.Name == "" || report.Manifest.Version == "" || report.Manifest.Prefix == "" {
		return nil, nil, fmt.Errorf("incomplete manifest in rulepack output (name, version, and prefix are required)")
	}

	provenance := fmt.Sprintf("%s@%s", report.Manifest.Name, report.Manifest.Version)

	diags := make([]diagnostics.Diagnostic, 0, len(report.Findings))
	for _, f := range report.Findings {
		d := f.Diagnostic
		d.Source = provenance
		diags = append(diags, d)
	}

	// Los descriptores son best-effort: si vienen malformados se ignoran y se
	// conservan los findings, en vez de perder todo el reporte del pack.
	var descriptors []RuleDescriptor
	if len(report.Descriptors) > 0 {
		if err := json.Unmarshal(report.Descriptors, &descriptors); err != nil {
			descriptors = nil
		}
	}

	return diags, descriptors, nil
}
