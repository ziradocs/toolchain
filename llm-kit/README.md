# ZiraDocs / DocLang — LLM Kit

This kit teaches any LLM (or agent) to emit **valid** ZiraDocs
(`.slidelang`, presentations) and DocLang (`.doclang`, documents) source
— bring your own model, no dependency on a specific provider or hosted
service.

This is the **open, BYO-LLM** kit: calibrated system prompts, syntax
reference, and a validation checklist you can run yourself. Managed
generation — further-tuned prompts, evals, and model selection as a
hosted service — is a separate, commercial offering; this kit doesn't
depend on it and doesn't include it.

## Start here

- **`system-prompt.md`** — the main system prompt. Use this if you have
  room for it.
- **`system-prompt-compact.md`** — a <8k-character condensed variant, for
  contexts with a tight system-prompt budget (e.g. some GPT/agent
  builders).

Both cover the same ground; keep them in sync if you edit either.

## The three real validity targets

ZiraDocs and DocLang share one parser/renderer core, but validity works
differently across them. There are exactly three combinations worth
targeting:

| Target | Structure | AI normalization | Validation tooling |
|---|---|---|---|
| **ZiraDocs strict** | Rigid `SLIDE <type>` keyword blocks | Never runs | `slidelang build --lint-only`, MCP `lint`/`get_ast` |
| **ZiraDocs flex** | Markdown-like, `#`/`##`/`---` | Always on (via the CLI) | Same as above |
| **DocLang flex** | Markdown section hierarchy (`#`/`##`/`###`) | Always on (via the CLI) | `doclang build --lint-only`, MCP `lint`/`get_ast` (element rules) |
| **DocLang strict** | `SECTION "Title"` blocks with `level:`/`id:` | Never | same as DocLang flex |

**DocLang strict is not SlideLang strict.** It is built from `SECTION`
blocks, never `SLIDE`, and `SLIDE` does not parse in a `.doclang` file at
all. Older design sketches described extra strict properties (`numbered:`,
`pagebreak:`) and block forms (`<<toc>>`, `<<ref:>>`, `<<bibliography>>`) —
**those remain unimplemented**; don't teach them as something that will
parse. `reference/doclang-strict.md` documents what actually does.

## Validation surfaces (read this before trusting any "it's valid" claim)

Both CLIs run the same linter. `slidelang build --lint-only` and `doclang
build --lint-only` parse and then lint (11 rule types producing 40+
distinct diagnostic codes across structural checks, strict-mode checks,
element checks, and 19 per-layout schemas — see `validation-checklist.md`
for the complete, rule-ID-mapped list), and both CLIs ship an MCP server
(`slidelang mcp`, `doclang mcp`) exposing `lint`, `get_ast`,
`list_themes`, `preview` over source held in memory.

The **element** rules fire identically on `.slidelang` and `.doclang`,
because both formats use the same element parsers: table column counts
(`TABLE003`, error), code groups (`CODEGROUP001/002`), image sources
(`IMG001`), empty code blocks (`CODE001`), charts (`CHART001/003/004`),
special-block types (`SPECIAL001`). A malformed table is caught in either
format. Two document-level rules also fire on both: `CORE001` (at least
one block) and `FRONT003` — the DocLang *parser* tolerates a file with no
frontmatter, but the linter does not, so give every `.doclang` file a
frontmatter block.

What is **not** symmetric:

- The slide-shaped rules — strict-mode checks (`STRICT001/002`), parse
  heuristics (`PARSE001/002`) and the 19 per-layout schemas
  (`validation-checklist.md` §2 and §5) — are written against the slide
  model. On a DocLang document they mostly don't apply; the layout schemas
  can produce cosmetic noise (a first section carrying body text draws a
  "Title slides typically should not contain content elements" warning).
  Warnings only; nothing there blocks a DocLang build.
- DocLang ignores frontmatter `mode:` entirely, so there is no mode choice
  to validate.
- `slidelang mcp`'s `lint` tool applies **no** lint policy, while
  `doclang mcp`'s resolves `--lint-config` > frontmatter `lint_policy:` >
  default exactly like the CLI does. In that one respect DocLang's tooling
  is ahead of ZiraDocs's.

## Kit contents

```
system-prompt.md              Main system prompt
system-prompt-compact.md      <8k-char condensed variant
reference/
  slidelang-strict.md          Strict-mode grammar, worked example
  slidelang-flex.md            Flex-mode grammar, AI normalizer scope
  doclang-flex.md               DocLang's inferred dialect, frontmatter gaps
  doclang-strict.md             DocLang's declared SECTION dialect
  elements.md                   Per-element syntax table (all 3 targets)
  frontmatter.md                Every recognized frontmatter field
  advanced.md                   Deep-dive chart/mermaid/layout examples
use-case-prompts/
  academic.md, business-pitch.md, education.md, creative.md
validation-checklist.md        Every linter rule ID, mapped to a checklist item
examples/README.md             Points at examples/gallery/ (the real corpus)
```

## Examples

This kit does not duplicate example decks/documents — see
`examples/README.md`, which points at the repository's
`examples/gallery/` as the single, lint-clean, dual-format source of
truth.

## Also useful

- `core/spec/language-specification.md` — the formal DSL
  specification.
- The MCP servers (`slidelang mcp`, `doclang mcp`; see
  `slidelang/internal/mcp/` and `doclang/internal/mcp/`) — each exposes
  `lint`, `get_ast`, `list_themes`, `preview` tools for programmatic
  validation.
- `../llms.txt` (repo root) — the machine-readable entry point for
  slidelang.org, per the llmstxt.org convention.
