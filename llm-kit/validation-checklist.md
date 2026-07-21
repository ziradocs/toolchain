# Validation Checklist — Par With The Linter

This checklist mirrors **every diagnostic code** the actual linter
(`core/linter/rules.go` + `layout_validation.go`) can produce, so
it stays auditable: each item names the rule ID that backs it. Run
through it before presenting final ZiraDocs output, or better, run the
real thing:

```bash
slidelang build your-file.slidelang --lint-only
```

or, via the MCP server, the `lint` tool (see `README.md`'s "Validation
surfaces" section). **DocLang runs the same linter** — `doclang build
your-file.doclang --lint-only`, or `doclang mcp`'s `lint` tool — but not
every item below applies to it: §3 carries over in full, §1 partly, and
§2, §4, §5, §6 are written against the slide model. See "For DocLang" at
the end.

## 1. Presentation-level structure

- [ ] `---` frontmatter present at file start (`FRONT003` — error if missing)
- [ ] `mode:` set explicitly in frontmatter (`FRONT001` — the **parser**
      emits this as a WARNING and backfills `mode: auto` if missing; the
      linter's own `FRONT001` check can never fire, because by the time it
      runs, `mode` has already been backfilled to `auto` — so a missing
      `mode` is a warning, not a build-blocking error. Still don't skip it:
      `auto` silently changes which grammar your file is parsed as)
- [ ] At least one slide exists (`CORE001` — error if the presentation has
      zero content blocks)
- [ ] No slide is completely empty — no title, no elements (`SLIDE002` —
      warning) — this also flags likely indentation mistakes (`SYNTAX001` —
      warning: "check syntax, content should be indented under SLIDE
      declaration")
- [ ] No two consecutive slides with zero elements (`PARSE001` — **error**:
      a strong signal of a parsing/structural bug, not just sparse content).
      This checks `len(Elements) == 0` only — a **title is not enough**: two
      adjacent title-only divider slides (title set, no `TEXT`/`POINTS`/etc.)
      still trip this. Give consecutive divider-style slides at least one
      element each, or separate them with real content.
- [ ] The document doesn't look like a single mis-parsed blob — i.e. not
      exactly one slide with no title and no elements (`PARSE002` —
      warning: "ensure content is properly indented")

## 2. Strict-mode-only rules (only apply when `mode: strict`)

- [ ] Every `title`-type slide has a `heading` or `title` property
      (`STRICT001` — error)
- [ ] Every `content`-type slide (or untyped slide) has either a `title` or
      at least one element (`STRICT002` — error)

## 3. Element structure

- [ ] **Tables**: has headers (`TABLE001` — warning), has at least one row
      (`TABLE002` — warning), and **every row has the same column count as
      the header row** (`TABLE003` — **error**, the strictest of the three)
- [ ] **Code groups**: contains at least one code block (`CODEGROUP001` —
      error); no `:::code-item{...}` tab left orphaned outside a real
      `:::code-group`/`::::code-group` wrapper (`CODEGROUP002` — error —
      this usually means the code-group normalization didn't run or
      failed)
- [ ] **Special blocks**: `BlockType` is one of exactly `info`, `warning`,
      `danger`, `success`, `tip`, `details` (`SPECIAL001` — warning for
      anything else, e.g. `note`, `error`, `poll`, `notes`)
- [ ] **Charts**: has data — either `data`/`series` YAML or a JSON payload
      (`CHART001` — warning if neither is present; column/series
      consistency is **not** checked by the linter, verify it yourself)
- [ ] **Images**: has a non-empty `source` (`IMG001` — error)
- [ ] **Code**: block is not empty (`CODE001` — warning)

## 4. Slide properties

- [ ] If a slide sets `logo:`, it points to a file with a valid image
      extension — `.png`, `.jpg`, `.jpeg`, `.gif`, `.svg`, `.webp`
      (`PROP001` — warning otherwise)

## 5. Per-layout schema (only applies when a slide has a recognized
   `layout`/`BlockType`)

Every layout type below has a schema: required properties, allowed vs.
forbidden element types, and min/max element counts. Violations surface as
`LAYOUT_MIN_ELEMENTS`, `LAYOUT_MAX_ELEMENTS`, `LAYOUT_FORBIDDEN_ELEMENT`
(all warnings), plus a layout-specific rule with its own code (see table).

| Layout | Required props | Allowed elements | Forbidden elements | Min/Max | Specific rule |
|---|---|---|---|---|---|
| `title` | `heading` | *(none — properties only)* | text, code, points, table, image | 0 / 0 | `LAYOUT001` heading required (error); `LAYOUT002` warns if content elements present |
| `title_slide` | `heading` | *(none)* | text, code, points, table | 0 / 1 | — |
| `content` | `title` | text, code, points, table, image, special_block, mermaid, chart, map, directive | *(none)* | 1 / ∞ | `LAYOUT003` missing title (warning); `LAYOUT004` no elements (**error**) |
| `section` | `title` | text, points | code, table, chart, map | 1 / 3 | `LAYOUT005` complex elements present (warning) |
| `comparison` | `title` | text, points, table, special_block | code, mermaid, chart | 2 / 4 | `LAYOUT006` <2 elements (warning) |
| `stats` | `title` | text, chart, table, special_block | code, mermaid | 1 / 3 | `LAYOUT007` no chart/table (warning) |
| `code_example` | `title` | text, code, points | table, chart, map | 1 / 4 | `LAYOUT008` no code block (**error**) |
| `hero` | `title` | text, image, special_block | code, table, chart | 0 / 3 | `LAYOUT009` no title (**error**) |
| `testimonial` | *(none)* | text, image, special_block | code, table, chart | 1 / 3 | `LAYOUT010` no quote/author signal (warning) |
| `timeline` | `title` | text, points, special_block | code, table, chart | 2 / 6 | `LAYOUT011` <2 events (warning) |
| `before_after` | `title` | text, image, points, special_block | code, chart | 2 / 4 | `LAYOUT012` missing before/after sections (warning) |
| `pricing` | `title` | text, table, special_block | code, chart, mermaid | 1 / 4 | `LAYOUT013` no plan/price signal (warning) |
| `team` | `title` | text, image, special_block | code, chart, table | 1 / 8 | `LAYOUT014` no member/role signal (warning) |
| `feature_showcase` | `title` | text, points, image, special_block | code, table | 2 / 6 | `LAYOUT015` <2 features (warning) |
| `call_to_action` | `title` | text, special_block | code, table, chart, mermaid | 1 / 3 | `LAYOUT016` no CTA signal (warning) |
| `dashboard` | `title` | text, chart, table, special_block | code, mermaid | 1 / 6 | `LAYOUT017` no metrics signal (warning) |
| `process` | `title` | text, points, special_block | code, table, chart | 2 / 6 | `LAYOUT018` <2 steps (warning) |
| `default` | *(none)* | text, code, points, table, image, special_block, mermaid, chart, map, directive | *(none)* | 0 / ∞ | — |
| `closing` | *(none)* | text, image, points | code, table, chart, mermaid, map | 0 / 3 | complex-element / >3-element warnings |

- [ ] Don't repeat the same specialized layout more than ~2 times
      consecutively (this is a style guideline from the original quality
      checklist, not a linter-enforced rule — no rule ID)

## 6. Auto-detection (informational, not something you need to prevent)

- [ ] The **last** slide, if it has no title/heading and its type is
      unset/`content`/`default`, is automatically re-typed as `closing`
      (`LAYOUT_AUTO_CLOSING` — info-level, not an error or warning; you
      don't need to do anything about this, just know it happens)

## 7. Content-quality guidelines (not linter-enforced — style only)

None of these have rule IDs; they're carried over from prior authoring
guidance because they still produce better decks, but the linter will not
flag their absence:

- [ ] Max ~5 bullet points per text-heavy slide
- [ ] One primary visual (chart/diagram) per slide, not several competing
      for attention
- [ ] Chart `series` length matches the number of numeric columns per
      `data` row, and units are consistent within a series
- [ ] Numeric consistency across slides (totals, percentages that should
      agree, do agree)
- [ ] Any `{{variable}}` placeholder used in content is defined in
      frontmatter `variables:` (or removed)

## 8. Output contract

- [ ] Final ZiraDocs output is presented as a single fenced
      ` ```slidelang ` block — no partial/duplicate frontmatter, no
      leftover bracket placeholders (`[CHART: ...]`, `[DIAGRAM: ...]`) in
      the final version
- [ ] No unsupported closing tags anywhere (`<</chart>>`, `<</mermaid>>`,
      `<</map>>`) and no unimplemented interactive tags (`<<poll>>`,
      `<<quiz>>`, `:::poll`, `:::qa_session`, `:::reveal`) — see
      `reference/elements.md`'s "no unsupported closing tags" section

## For DocLang: what's actually checked

`doclang build` runs the parser **and** the same linter, so most of this
list is not ZiraDocs-only:

- **Section 3 (element structure) applies in full.** DocLang uses the
  identical element parsers, so `TABLE003`, `CODEGROUP001/002`, `IMG001`,
  `CODE001`, `CHART001` and `SPECIAL001` fire on `.doclang` exactly as they
  do on `.slidelang`. A mismatched table row is an error in both.
- **Sections 2, 5 and 6 don't meaningfully apply.** The strict-mode rules
  never fire (DocLang ignores `mode:`), and the per-layout schemas are
  written for slides. They can still emit cosmetic warnings — a first
  section carrying body text draws "Title slides typically should not
  contain content elements". Those are warnings; they don't block a build
  and there's nothing to fix.
- **Section 1 partly applies.** `FRONT003` (frontmatter present) and
  `CORE001` (at least one block) fire on `.doclang` too — despite the
  DocLang parser tolerating a frontmatter-less file, the linter that
  `doclang build` runs rejects one with an error, so give every `.doclang`
  file at least a minimal `---` block. The slide-shaped items in that
  section (`SLIDE002`, `SYNTAX001`, `PARSE001/002`) are ZiraDocs
  concerns. **Section 4** (slide `logo:` property) is ZiraDocs-only.
- **Section 8's output contract** applies with ` ```doclang ` as the fence.

Plus the DocLang-specific reading: frontmatter (if present) is well-formed
YAML between `---` fences, and every `#`/`##`/`###` reads as intended given
the section-vs-nested-heading rule (see `reference/doclang-flex.md`) —
neither of which the linter checks for you.

Run it, the same way you would for ZiraDocs:

```bash
doclang build your-file.doclang --lint-only
```
