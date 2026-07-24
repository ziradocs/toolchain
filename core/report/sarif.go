package report

import (
	"encoding/json"

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

	for _, d := range active {
		run.Results = append(run.Results, toSarifResult(d, nil, docPath))
	}
	for _, w := range waived {
		run.Results = append(run.Results, toSarifResult(w.Diagnostic, w.Policy, docPath))
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
						Uri: docPath,
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
