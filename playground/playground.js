// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Thin wrapper over the wasm-exported functions (cmd/wasm/*.go). Every
// export takes/returns strings — structured results are JSON, parsed here.
// No build step, no bindings generator: this is the entire JS surface.

const SAMPLES = {
  "slidelang-strict": `---
mode: strict
title: "Playground Strict Sample"
---

SLIDE title
  heading: "SlideLang Strict Mode"
  subtitle: "Try editing this file"

SLIDE content
  title: "Why Strict Mode?"
  TEXT
    Strict mode trades flexibility for guarantees: every slide type and
    field is declared explicitly with SLIDE blocks.
  POINTS
    - Every slide starts with SLIDE <layout>
    - No ambiguity between headings and content

SLIDE closing
  title: "Thanks for Reading"
`,
  "slidelang-flex": `---
mode: flex
title: "Playground Flex Sample"
theme: "modern-blue"
---

# SlideLang Flex Mode
## Markdown-like authoring

Edit this content and watch the preview update.

---

## A Chart

<<chart: bar>>
  data: [
    ["Q1", 45, 32],
    ["Q2", 52, 38]
  ]
  series: ["Product A", "Product B"]

---

## A Diagram

<<mermaid>>
  graph TD
      A[Start] --> B{Decision}
      B -->|Yes| C[Path 1]
      B -->|No| D[Path 2]
`,
  doclang: `---
title: "Playground Document Sample"
author: "You"
---

# Executive Summary

This is a DocLang document. Unlike SlideLang, frontmatter is optional and
` + "`#`" + ` headings start new sections while ` + "`##`/`###`" + ` nest inside them.

::: success
Edit this content and watch the preview update.
:::

## Key Metrics

| Metric | Value |
|--------|-------|
| Uptime | 99.98% |
| Latency | 42ms |
`,
};

let wasmReady = null;

function loadWasm() {
  if (wasmReady) return wasmReady;
  wasmReady = (async () => {
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(
      fetch("slidelang.wasm"),
      go.importObject
    );
    go.run(result.instance); // does not return — the Go side blocks in select{}
  })();
  return wasmReady;
}

function parseJSON(raw, label) {
  try {
    return JSON.parse(raw);
  } catch (e) {
    return { error: `${label}: invalid JSON response (${e.message})` };
  }
}

function renderDiagnostics(el, diags) {
  el.innerHTML = "";
  if (!diags || diags.length === 0) {
    el.innerHTML = '<div class="diag-empty">No diagnostics.</div>';
    return;
  }
  for (const d of diags) {
    const row = document.createElement("div");
    row.className = `diag diag-${(d.severity || "info").toLowerCase()}`;
    const loc = d.position ? `${d.position.line}:${d.position.column}` : "";
    row.textContent = `[${d.severity}] ${loc} ${d.message}${d.ruleId ? " (" + d.ruleId + ")" : ""}`;
    el.appendChild(row);
  }
}

async function populateThemes(select) {
  const raw = window.slidelangListThemes();
  const out = parseJSON(raw, "slidelangListThemes");
  select.innerHTML = "";
  const blank = document.createElement("option");
  blank.value = "";
  blank.textContent = "(document default)";
  select.appendChild(blank);
  for (const name of out.themes || []) {
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    select.appendChild(opt);
  }
}

function runRender(state) {
  const { sourceEl, formatEl, themeEl, previewEl, diagEl, statusEl } = state;
  const source = sourceEl.value;
  const format = formatEl.value;
  const theme = themeEl.value;

  let raw;
  if (format === "doclang") {
    raw = window.doclangRenderHTML(source, theme);
  } else {
    raw = window.slidelangRenderSlides(source, theme);
  }
  const out = parseJSON(raw, "render");

  renderDiagnostics(diagEl, out.diagnostics);

  if (out.error) {
    statusEl.textContent = `Error: ${out.error}`;
    statusEl.className = "status status-error";
    previewEl.srcdoc = "";
    return;
  }
  if (!out.valid) {
    statusEl.textContent = "Parse errors — see diagnostics below.";
    statusEl.className = "status status-error";
    previewEl.srcdoc = "";
    return;
  }
  statusEl.textContent = "OK";
  statusEl.className = "status status-ok";
  previewEl.srcdoc = out.html;
}

function runLint(state) {
  const { sourceEl, formatEl, diagEl, statusEl } = state;
  if (formatEl.value === "doclang") {
    statusEl.textContent = "DocLang has no linter — only parse errors are shown (see the LLM Kit README).";
    statusEl.className = "status status-info";
    return;
  }
  const raw = window.slidelangLint(sourceEl.value);
  const out = parseJSON(raw, "lint");
  renderDiagnostics(diagEl, out.diagnostics);
  statusEl.textContent = out.valid ? "Lint: valid" : "Lint: has errors";
  statusEl.className = out.valid ? "status status-ok" : "status status-error";
}

window.addEventListener("DOMContentLoaded", async () => {
  const state = {
    sourceEl: document.getElementById("source"),
    formatEl: document.getElementById("format"),
    themeEl: document.getElementById("theme"),
    previewEl: document.getElementById("preview"),
    diagEl: document.getElementById("diagnostics"),
    statusEl: document.getElementById("status"),
  };
  const sampleEl = document.getElementById("sample");
  const lintBtn = document.getElementById("lint-btn");

  state.statusEl.textContent = "Loading WebAssembly runtime...";
  try {
    await loadWasm();
  } catch (e) {
    console.error("Failed to load slidelang.wasm:", e);
    state.statusEl.textContent =
      "Failed to load WebAssembly runtime — check the console (common cause: " +
      "the server isn't sending application/wasm for .wasm, see README.md).";
    state.statusEl.className = "status status-error";
    return;
  }
  state.statusEl.textContent = "Ready.";

  await populateThemes(state.themeEl);

  function loadSample() {
    const key = state.formatEl.value === "doclang" ? "doclang" : sampleEl.value;
    state.sourceEl.value = SAMPLES[key] || SAMPLES["slidelang-flex"];
    runRender(state);
  }

  sampleEl.addEventListener("change", loadSample);
  state.formatEl.addEventListener("change", () => {
    sampleEl.disabled = state.formatEl.value === "doclang";
    state.themeEl.disabled = state.formatEl.value === "doclang";
    loadSample();
  });
  state.themeEl.addEventListener("change", () => runRender(state));
  lintBtn.addEventListener("click", () => runLint(state));

  let debounce;
  state.sourceEl.addEventListener("input", () => {
    clearTimeout(debounce);
    debounce = setTimeout(() => runRender(state), 300);
  });

  loadSample();
});
