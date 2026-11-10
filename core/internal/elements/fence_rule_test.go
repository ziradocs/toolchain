// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// Una línea que empieza con ``` pero no abre una cerca (internal/fence) no
// abre otro bloque: continúa un item y no corta la prosa.
func TestFenceRule_BacktickInfoLineIsNotABlock(t *testing.T) {
	if StartsOtherBlock("```js` sigue") {
		t.Error("StartsOtherBlock must not take ```js` as a fence")
	}
	if !StartsOtherBlock("```go") || !StartsOtherBlock("````md") {
		t.Error("StartsOtherBlock must take a real fence")
	}
	if IsNewElement("```js` sigue", "flex") {
		t.Error("IsNewElement must not take ```js` as a code block")
	}
	if !IsNewElement("````md", "flex") {
		t.Error("IsNewElement must take ````md as a code block")
	}
}
