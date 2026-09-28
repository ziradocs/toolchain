// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/linter"
)

const pollWithResults = `---
mode: strict
---
SLIDE content
  title: "S"
  <<poll>>
    question: "Preferred plan?"
    options: ["Basic", "Pro", "Team"]
    results: [20, 45.5, 34.5]
    responses: 120
  <<end>>
  <<quiz>>
    question: "Capital of France?"
    options: ["Paris", "Rome"]
    answer: 0
    results: [80, 30]
  <<end>>
`

// results/responses entran con el opt-in, llevan el documento a 2.20.0 y
// sobreviven a fmt; el linter no exige que los porcentajes sumen 100.
func TestQuizPollResultsRoundTrip(t *testing.T) {
	doc := mustParse(t, pollWithResults, false)
	poll := firstOfType[*ast.PollElement](t, doc)
	if !reflect.DeepEqual(poll.Results, []float64{20, 45.5, 34.5}) || poll.Responses == nil || *poll.Responses != 120 {
		t.Fatalf("poll = %+v", poll)
	}
	if doc.SchemaVersion != ast.QuizPollResultsSchemaVersion || !reflect.DeepEqual(doc.Capabilities, []string{ast.QuizPollResultsCapability}) {
		t.Fatalf("contract = %s %v", doc.SchemaVersion, doc.Capabilities)
	}
	for _, d := range linter.New().Lint(doc) {
		if d.RuleID == "QUIZ003" || d.RuleID == "POLL003" {
			t.Fatalf("valid results reported: %v", d)
		}
	}
	out, err := FormatStrict(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "ast_capabilities") || !strings.Contains(out, "results: [20, 45.5, 34.5]") {
		t.Fatalf("fmt output:\n%s", out)
	}
	again := firstOfType[*ast.PollElement](t, mustParse(t, out, false))
	if !reflect.DeepEqual(again.Results, poll.Results) || *again.Responses != 120 {
		t.Fatalf("round trip = %+v", again)
	}
}

// La capability se infiere de la sintaxis nueva: no hace falta declararla, y
// declararla en frontmatter es un error, igual que table-rows-v1.
func TestQuizPollResultsCapabilityIsInferred(t *testing.T) {
	src := strings.Replace(pollWithResults, "mode: strict\n", "mode: strict\nast_capabilities: [quiz-poll-results-v1]\n", 1)
	_, diags := parseAny(src)
	found := false
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, "quiz-poll-results-v1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("frontmatter declaration of an inferred capability accepted: %v", diags)
	}
}

// El linter valida longitud y rango.
func TestQuizPollResultsLint(t *testing.T) {
	for _, tc := range []struct{ results, responses string }{
		{"[20, 80, 0, 0]", ""},
		{"[20, 120, 0]", ""},
		{"[20, -1, 81]", ""},
	} {
		src := "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<poll>>\n    question: \"Q?\"\n    options: [\"A\", \"B\", \"C\"]\n    results: " + tc.results + "\n  <<end>>\n"
		doc, _ := parseAny(src)
		if doc == nil {
			t.Fatalf("no AST for %s", tc.results)
		}
		found := false
		for _, d := range linter.New().Lint(doc) {
			if d.RuleID == "POLL003" {
				found = true
			}
		}
		if !found {
			t.Errorf("results %s not reported", tc.results)
		}
	}
}
