# Generating with LLMs / AI Agents

You don't need a specific AI product to generate ZiraDocs or DocLang —
any LLM/agent can emit valid source if it's given the right system prompt
and told how to validate its own output. This is the **bring-your-own-model**
path; the `llm-kit/` directory at the repository root has everything you
need.

## Quick start

1. **Pick a system prompt** and paste it into your model/agent's system
   prompt:
   - [`llm-kit/system-prompt.md`](../../../llm-kit/system-prompt.md) — the
     main prompt, use this if you have room for it.
   - [`llm-kit/system-prompt-compact.md`](../../../llm-kit/system-prompt-compact.md) —
     a <8k-character condensed variant, for contexts with a tight
     system-prompt budget (e.g. some GPT/agent builders).
2. **Ask for one of the three real validity targets** (see table below) —
   don't mix them in one request.
3. **Validate the output for real**, don't just trust the model:
   - ZiraDocs: `slidelang build --lint-only`, or the MCP server's `lint`/
     `get_ast` tools.
   - DocLang: `doclang build` and check for parser errors — there is no
     linter for DocLang (see the asymmetry below).

## The three real validity targets

ZiraDocs and DocLang share one parser/renderer core, but validity works
differently across them. There are exactly three combinations worth
targeting — never ask a model for a "DocLang strict" syntax, since the
parser doesn't have one:

| Target | Structure | AI normalization | Validation tooling |
|---|---|---|---|
| **ZiraDocs strict** | Rigid `SLIDE <type>` keyword blocks | Never runs | `slidelang build --lint-only`, MCP `lint`/`get_ast` |
| **ZiraDocs flex** | Markdown-like, `#`/`##`/`---` | Always on (via the CLI) | Same as above |
| **DocLang flex** | Markdown section hierarchy (`#`/`##`/`###`) | Always on (via the CLI) | **None** |

The DocLang parser always uses its flex/section grammar regardless of any
`mode:` frontmatter value, and there is no strict mode or linter for it at
all — a `DOCLANG_SYNTAX_STRICT.md` document exists describing one, but it's
aspirational, not implemented.

## The validation asymmetry (read this before trusting "it's valid")

ZiraDocs has real, automatable validation: `slidelang build --lint-only`
runs the parser and the full linter (11 rule types producing 40+ distinct
diagnostic codes, plus 19 per-layout schemas). The MCP server also exposes
`lint` and `get_ast` tools that do the same thing programmatically — see
[`llm-kit/validation-checklist.md`](../../../llm-kit/validation-checklist.md)
for the complete, rule-ID-mapped list.

**DocLang has none of this.** `doclang build` runs the parser only — no
`--lint-only` equivalent, and the MCP `lint`/`get_ast` tools only operate on
`.slidelang` source. For DocLang, "valid" means exactly one thing: *the
parser produced output without an error-level diagnostic.* Be more
conservative with DocLang element usage than you might be tempted to be
with ZiraDocs, since there's no automated second opinion catching a
mistake.

## Two things worth knowing before you start prompting

- **Flex mode has no per-slide layout typing.** A `---\nlayout: <type>\n---`
  block in flex-mode ZiraDocs does nothing — the parser discards it as
  inert metadata. If you need a specific layout's schema validation
  (comparison, stats, timeline, etc.), ask for **strict mode**'s
  `SLIDE <type>` instead.
- **The AI normalizer isn't a validity crutch.** It rewrites loose
  Markdown-ish flex content into canonical elements, but it never runs in
  strict mode and won't invent missing data or fix a chart's series/column
  mismatch — get the syntax right, don't rely on it being repaired.

## Go deeper

- [`llm-kit/README.md`](../../../llm-kit/README.md) — the kit's own
  overview and full contents list.
- [`llm-kit/reference/`](../../../llm-kit/reference/) — per-target grammar
  (`slidelang-strict.md`, `slidelang-flex.md`, `doclang-flex.md`),
  per-element syntax (`elements.md`), frontmatter fields
  (`frontmatter.md`), and deeper worked examples (`advanced.md`).
- [`llm-kit/use-case-prompts/`](../../../llm-kit/use-case-prompts/) —
  calibrated skeletons for academic, business-pitch, education, and
  creative decks.
- [`llms.txt`](../../../llms.txt) — the machine-readable entry point
  (llmstxt.org convention) linking the authoritative docs a model/agent
  might want to fetch directly.
- [`examples/gallery/`](../../../examples/gallery/) — the real, lint-clean,
  dual-format example corpus the kit's use-case prompts reference instead
  of duplicating.
