// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

'use strict';

const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const MODULES_DIR = path.join(__dirname, '..', 'assets', 'js', 'modules');

// loadModule evaluates a module file from assets/js/modules UNMODIFIED in
// a fresh vm context, against minimal stubs — never a copy, never a
// require()-friendly rewrite, so tests exercise exactly the source that
// ships to the browser. These files are plain <script> tags with no module
// wrapper (no CommonJS/ESM), which is why vm.runInContext is used instead
// of require().
//
// Returns the vm context. Top-level `function` declarations in the loaded
// script become own-properties of the context (standard script-execution
// hoisting) and are reachable as `ctx.someFunction`. Top-level `const`/
// `let` do NOT become context properties — deliberately not worked around
// here, since every function this harness's tests need (withAlpha,
// scaleDimension, ensureThemeableScales, applyExtensionChartColors,
// categoricalColor, ...) is a `function` declaration, not a const.
function loadModule(filename) {
    const source = fs.readFileSync(path.join(MODULES_DIR, filename), 'utf8');
    const sandbox = {
        // SlideLang is read directly (not via window.SlideLang) by the
        // functions under test — tests mutate ctx.SlideLang.metadata
        // between cases to inject theme tokens. registerModule is a
        // no-op stub: charts.js's own registration IIFE calls
        // SlideLang.registerModule('charts', SlideLangCharts) once
        // window/document are wired up (below) — without this it throws
        // before any test body runs, since our stub SlideLang has no
        // real module registry behind it.
        SlideLang: { metadata: {}, registerModule: () => {} },
        // window = {} (present, not self-referential to the sandbox) so
        // that: (a) the module's trailing, unconditional
        // `window.SlideLangCharts = SlideLangCharts` doesn't throw, and
        // (b) `typeof window !== 'undefined'` is true, taking the
        // registration IIFE's synchronous init() path instead of its
        // setTimeout-retry fallback — which would otherwise leave a
        // dangling timer if setTimeout weren't also stubbed as a no-op.
        window: {},
        // charts.js's registration IIFE checks document.readyState
        // unconditionally (not guarded by typeof), and — once
        // window/SlideLang are wired up as above — init() calls
        // document.addEventListener and document.querySelector via
        // processInitialSlide(). querySelector returns null (no active
        // slide) so processInitialSlide's body — which would otherwise
        // walk real DOM state this harness has no reason to fake — never
        // executes.
        document: {
            readyState: 'complete',
            addEventListener: () => {},
            querySelector: () => null,
        },
        // Only reached if window/SlideLang above were somehow NOT wired
        // up; kept as a harmless no-op rather than left undefined so a
        // future edit to this harness can't leave a dangling real timer.
        setTimeout: () => {},
        console,
    };
    vm.createContext(sandbox);
    vm.runInContext(source, sandbox, { filename });
    return sandbox;
}

module.exports = { loadModule };
