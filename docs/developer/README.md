# Developer Documentation

Technical documentation for developers working on `core`, `slidelang`, or
`doclang`, or building integrations against them.

**Start with the root [`CLAUDE.md`](../../CLAUDE.md)** — it's the single up-to-date architecture
overview (module layout, the parsing pipeline, the element registry, rendering, the content
normalizer) and is kept current as the codebase changes. The pages below cover specific systems
in more depth than `CLAUDE.md`'s summary.

## Implementation guides

- **[docxgo fork](docxgo-fork.md)** — why `doclang`'s DOCX export depends on a personal fork,
  and what diverges from upstream.

> Guides for the Chromium integration, offline PlantUML rendering and the theme system used to
> live here. They had drifted far enough from the code to be actively misleading — documenting
> CLI flags that no longer exist (`--mermaid-mode`, `--plantuml-mode`, both long since folded
> into `--render-mode`), pre-monorepo `src/` paths, and an install location the installer never
> used — so they were removed rather than left to mislead. The source of truth for these systems
> is the code itself: `core/renderer/chromium/`, `core/renderer/chromium/plantuml_fetcher.go`,
> and `slidelang/internal/generator/css/`.

## Specifications and contracts

- **[Language Specification](../../core/spec/language-specification.md)** — the formal
  DSL grammar (part of the [Spec v0.1 index](../../core/spec/)).
- **[JSON/AST contract](../architecture/json-ast-contract.md)** — what `--format json` emits,
  field by field, and its semver compatibility policy.
- **[HTML sanitization](../architecture/sanitization.md)** — the sanitizer's guarantees.

## Contributing

See the root [`CONTRIBUTING.md`](../../CONTRIBUTING.md) for the DCO requirement, multi-module
build/test setup, and code conventions.
