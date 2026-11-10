// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const { loadModule } = require('./harness.js');

// process corre SlideLangMermaid.miniminalProcessing sobre content. El módulo
// declara SlideLangMermaid con const, así que no queda como propiedad del
// contexto: se lee con otro script en el mismo contexto.
function process(content) {
    const ctx = loadModule('mermaid.js');
    ctx.input = content;
    return vm.runInContext('SlideLangMermaid.miniminalProcessing(input)', ctx);
}

test('miniminalProcessing keeps the relative indentation of a mindmap', () => {
    assert.equal(
        process('mindmap\n  root((Proyecto))\n    Alcance\n    Riesgos'),
        'mindmap\n  root((Proyecto))\n    Alcance\n    Riesgos');
});

test('miniminalProcessing removes only the indentation shared by every line', () => {
    assert.equal(
        process('    mindmap\n      root((P))\n        A\n'),
        'mindmap\n  root((P))\n    A');
});

test('miniminalProcessing turns tabs into four spaces and keeps the depth', () => {
    assert.equal(
        process('mindmap\n\troot((P))\n\t\tA'),
        'mindmap\n    root((P))\n        A');
});

test('miniminalProcessing keeps the YAML frontmatter nesting', () => {
    assert.equal(
        process('---\nconfig:\n  theme: forest\n---\nflowchart LR\n  A --> B'),
        '---\nconfig:\n  theme: forest\n---\nflowchart LR\n  A --> B');
});

test('miniminalProcessing still drops blank lines, trailing spaces and CRLF', () => {
    assert.equal(
        process('flowchart LR  \r\n\r\n  A --> B \r\n   \r\n  B --> C\r\n'),
        'flowchart LR\n  A --> B\n  B --> C');
});
