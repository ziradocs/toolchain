# Examples

This kit deliberately does not maintain its own copy of example
decks/documents. The single source of truth is the repository's
**`examples/gallery/`** directory — 15 curated, English, lint-clean
examples covering both formats and all validity targets:

**ZiraDocs** (`.slidelang`):
- `01_strict_mode_basics.slidelang` — strict-mode grammar tour
- `02_flex_mode_essentials.slidelang` — flex-mode Markdown-like authoring
- `03_flex_ai_normalizer.slidelang` — what the AI normalizer rewrites
- `04_charts_showcase.slidelang` — chart types
- `05_mermaid_diagrams.slidelang` — mermaid diagram types
- `06_maps_and_geodata.slidelang` — the map element
- `07_special_blocks_and_checklists.slidelang` — info/warning/etc. + checklists
- `08_code_and_code_groups.slidelang` — code and code-group syntax
- `09_grid_layouts_and_tables.slidelang` — grid layout + tables
- `10_startup_pitch_deck.slidelang` — a full worked pitch deck

**DocLang** (`.doclang`):
- `01_business_report_basics.doclang`
- `02_technical_architecture.doclang`
- `03_academic_paper.doclang`
- `04_analytics_charts_and_maps.doclang`
- `05_product_onepager.doclang`

Every file in this directory builds and lints with zero errors under its
own CLI (`slidelang build --lint-only` / `doclang build --lint-only`) —
see `../validation-checklist.md` for what those runs check. Reference
these directly rather than copying excerpts into this kit, so the kit
never drifts out of sync with the actual corpus.
