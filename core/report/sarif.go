package report

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

type SARIF struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

type Tool struct {
	Driver Driver `json:"driver"`
}

type Driver struct {
	Name           string                `json:"name"`
	InformationUri string                `json:"informationUri"`
	Rules          []ReportingDescriptor `json:"rules,omitempty"`
}

// ReportingDescriptor modela el campo SARIF 2.1.0 driver.rules[] — el hogar
// estándar de metadata por regla (helpUri, properties). Se deriva de
// linter.RuleDescriptor, la fuente única definida una vez en core/linter.
type ReportingDescriptor struct {
	Id         string         `json:"id"`
	Name       string         `json:"name,omitempty"`
	HelpUri    string         `json:"helpUri,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

type Result struct {
	RuleId       string        `json:"ruleId"`
	Level        string        `json:"level"`
	Message      Message       `json:"message"`
	Locations    []Location    `json:"locations"`
	Suppressions []Suppression `json:"suppressions,omitempty"`
}

type Message struct {
	Text string `json:"text"`
}

type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

type ArtifactLocation struct {
	Uri string `json:"uri"`
}

type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

type Suppression struct {
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	Justification string `json:"justification,omitempty"`
}

func generateSARIF(active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic, docPath string, descriptors []linter.RuleDescriptor) ([]byte, error) {
	var rules []ReportingDescriptor
	if len(descriptors) > 0 {
		rules = make([]ReportingDescriptor, 0, len(descriptors))
		for _, d := range descriptors {
			rules = append(rules, ReportingDescriptor{
				Id:         d.ID,
				Name:       d.Name,
				HelpUri:    d.HelpURI,
				Properties: d.Properties,
			})
		}
	}

	run := Run{
		Tool: Tool{
			Driver: Driver{
				Name:           "ZiraDocs Toolchain",
				InformationUri: "https://ziradocs.com",
				Rules:          rules,
			},
		},
		Results: make([]Result, 0, len(active)+len(waived)),
	}

	for _, f := range orderedFindings(active, waived) {
		run.Results = append(run.Results, toSarifResult(f.diag, f.policy, docPath))
	}

	report := SARIF{
		Version: "2.1.0",
		Schema:  "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json",
		Runs:    []Run{run},
	}

	return json.MarshalIndent(report, "", "  ")
}

func toSarifResult(d diagnostics.Diagnostic, w *linter.RulePolicy, docPath string) Result {
	level := "note"
	if d.IsError() {
		level = "error"
	} else if d.Severity == diagnostics.Warning {
		level = "warning"
	}

	ruleId := d.RuleID
	if ruleId == "" {
		ruleId = d.Code
	}

	var region *Region
	if d.Position.Line > 0 {
		region = &Region{
			StartLine:   d.Position.Line,
			StartColumn: d.Position.Column,
		}
	}

	uri := artifactURI(docPath, d.File)

	res := Result{
		RuleId: ruleId,
		Level:  level,
		Message: Message{
			Text: d.Message,
		},
		Locations: []Location{
			{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{
						Uri: uri,
					},
					Region: region,
				},
			},
		},
	}

	if w != nil {
		sup := Suppression{
			Kind:   "external",
			Status: "accepted",
		}
		if w.Reason != "" {
			sup.Justification = w.Reason
		}
		res.Suppressions = []Suppression{sup}
	}

	return res
}

// artifactURI es el artifactLocation.uri de un diagnóstico: el documento
// (docPath), o el archivo incluido de Diagnostic.File. File viene relativo al
// directorio del documento (include.SourceMap.MapDiagnostics), así que se une
// a ese directorio: las dos uris quedan relativas a la misma base que docPath
// y ninguna necesita uriBaseId. No depende del sistema: "\" y "/" valen
// igual como separador en docPath y en File.
func artifactURI(docPath, file string) string {
	doc := slashed(docPath)
	if file == "" {
		return uriReference(doc)
	}
	f := slashed(file)
	if doc == "" || isAbsSlashed(f) {
		return uriReference(f)
	}
	joined := path.Join(path.Dir(doc), f)
	// path.Join deja una sola "/" al principio: una ruta UNC
	// ("//servidor/r/…") perdería el host.
	if strings.HasPrefix(doc, "//") && !strings.HasPrefix(joined, "//") {
		joined = "/" + joined
	}
	return uriReference(joined)
}

// slashed es p con "/" como separador.
func slashed(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// hasDrive dice si p empieza con una letra de unidad de Windows ("D:/…").
func hasDrive(p string) bool {
	if len(p) < 2 || p[1] != ':' || (len(p) > 2 && p[2] != '/') {
		return false
	}
	c := p[0]
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isAbsSlashed(p string) bool {
	return strings.HasPrefix(p, "/") || hasDrive(p)
}

// uriReference es la ruta p (con "/") como referencia URI (RFC 3986), con
// cada segmento escapado (espacios, Unicode, "%", "#", "?"). Una ruta
// absoluta va como URI file: "/srv/a b" es "file:///srv/a%20b",
// "D:/docs/x" es "file:///D:/docs/x" y "//servidor/r/x" (UNC) es
// "file://servidor/r/x". Una relativa queda relativa; si su primer segmento
// tiene ":", va con "./" para que no se lea como esquema.
func uriReference(p string) string {
	if p == "" {
		return ""
	}
	switch {
	case hasDrive(p):
		return "file:///" + escapeSegments(p)
	case strings.HasPrefix(p, "//"):
		return "file:" + escapeSegments(p)
	case strings.HasPrefix(p, "/"):
		return "file://" + escapeSegments(p)
	}
	if first, _, _ := strings.Cut(p, "/"); strings.Contains(first, ":") {
		p = "./" + p
	}
	return escapeSegments(p)
}

func escapeSegments(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}
