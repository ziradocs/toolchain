// Command reader_probe demonstrates the behavior of the previously published
// core/v2.37.0 JSON reader when given a new opt-in AST. Run with GOWORK=off
// from the doclang module, whose go.mod pins that published core release.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"go.ziradocs.com/core/v2/ast"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	doc, err := ast.DecodeAST(input)
	if err != nil {
		fmt.Printf("accepted=false error=%v\n", err)
		return
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	fmt.Printf("accepted=true schemaVersion=%s subListTypePreserved=%t\n", doc.SchemaVersion, bytes.Contains(encoded, []byte(`"subListType"`)))
}
