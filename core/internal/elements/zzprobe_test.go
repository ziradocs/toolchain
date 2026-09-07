package elements

import (
	"fmt"
	"strings"
	"testing"
)

func run(t *testing.T, name, src string) {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "<<quiz>>" || strings.TrimSpace(l) == "<<poll>>" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s: no tag", name)
	}
	tag := "quiz"
	if strings.TrimSpace(lines[start]) == "<<poll>>" {
		tag = "poll"
	}
	raw, consumed := readQuizPollBody(lines, start, tag)
	fmt.Printf("=== %s ===\nconsumed=%d (total lines from tag = %d)\nraw=%q\ndedent=%q\nLEFTOVER=%q\n\n",
		name, consumed, len(lines)-start, raw, dedentBlock(raw), lines[min(start+consumed, len(lines)):])
}


func TestProbe(t *testing.T) {
	run(t, "EOF sin cerrador", "<<quiz>>\nquestion: q\noptions:\n  - a\n  - b\nanswer: 0")
	run(t, "end dentro de string", "<<quiz>>\nquestion: \"termina con <<end>>\"\noptions:\n  - a\n  - b\n<<end>>\ndespues")
	run(t, "block scalar con <<end>> adentro", "<<quiz>>\nquestion: |\n  linea1\n  <<end>>\n  linea3\noptions:\n  - a\n  - b\n<<end>>\ndespues")
	run(t, "block scalar con << adentro", "<<quiz>>\nquestion: |\n  usa <<chart>> asi\n  <<chart: bar>>\noptions:\n  - a\n<<end>>\ndespues")
	run(t, "block scalar con # adentro", "<<quiz>>\nquestion: |\n  # titulo markdown\n  texto\noptions:\n  - a\n<<end>>\ndespues")
	run(t, "primera linea en blanco", "<<quiz>>\n\nquestion: q\noptions:\n  - a\n<<end>>\ndespues")
	run(t, "tabs", "<<quiz>>\n\tquestion: q\n\toptions:\n\t\t- a\n\t\t- b\n<<end>>")
	run(t, "options mapa anidado", "<<poll>>\nquestion: q\noptions:\n  a: 1\n  b: 2\n<<end>>\ndespues")
	run(t, "legacy closer", "<<quiz>>\nquestion: q\noptions:\n  - a\n  - b\nanswer: 1\n<</quiz>>\ndespues")
	run(t, "dedent inverso", "<<quiz>>\n  question: q\noptions:\n  - a\n<<end>>\ndespues")
	run(t, "--- separador", "<<quiz>>\nquestion: q\n---\nsiguiente slide")
	run(t, "yaml roto", "<<quiz>>\nquestion: [unclosed\n<<end>>\ndespues")
	run(t, "strict indentado", "  <<quiz>>\n    question: q\n    options:\n      - a\n      - b\n    answer: 0\n  <<end>>\n  otro")
}
