# ZiraDocs / DocLang — Documentation

The user-facing documentation now lives at **[ziradocs.com/docs](https://ziradocs.com/docs/)**
(Starlight, built from the separate `ziradocs/website` repo). The migration is done: the
`docs/user/getting-started/`, `docs/user/language-reference/`, `docs/user/features/`,
`docs/user/theme-implementation/`, `docs/doclang/` and `docs/architecture/` directories this
file used to index no longer exist here.

## Where things are now

| What | Where |
|---|---|
| Getting started, quickstart | [ziradocs.com/docs/slidelang/getting-started/quickstart](https://ziradocs.com/docs/slidelang/getting-started/quickstart/) |
| Language reference (SlideLang) | [ziradocs.com/docs/slidelang/language-reference/syntax-overview](https://ziradocs.com/docs/slidelang/language-reference/syntax-overview/) |
| Features, themes, variables | [ziradocs.com/docs/slidelang/features](https://ziradocs.com/docs/slidelang/features/) |
| DocLang | [ziradocs.com/docs/doclang/overview](https://ziradocs.com/docs/doclang/overview/) |
| Theme creation | [ziradocs.com/docs/slidelang/theme-implementation/theme-creation](https://ziradocs.com/docs/slidelang/theme-implementation/theme-creation/) |
| JSON/AST contract | [ziradocs.com/docs/architecture/json-ast-contract](https://ziradocs.com/docs/architecture/json-ast-contract/) |
| Sanitization | [ziradocs.com/docs/architecture/sanitization](https://ziradocs.com/docs/architecture/sanitization/) |
| Offline rendering & diagram backends | [ziradocs.com/docs/architecture/offline-rendering](https://ziradocs.com/docs/architecture/offline-rendering/) |

`docs.ziradocs.com` redirects to the same pages, so either host works — but `ziradocs.com` is
what Astro's `site` declares, and the one to write in new links.

## What stays in this repo

- **[Language specification](../core/spec/)** — the formal grammar and semantics (v0.1).
  Source of truth; the site's reference pages describe it, they don't replace it.
- **[Releasing](developer/releasing.md)** — the two release scripts and the core→CLIs dance.
- **[Guides](user/guides/)** — what hasn't moved yet.
- **[2026-07 security audit](SECURITY_AUDIT_2026-07.md)** — findings and their status.
- **[Contributing](../CONTRIBUTING.md)** · **[Security policy](../SECURITY.md)**

## Reporting a docs problem

Search or open an issue in the [issue tracker](https://github.com/ziradocs/toolchain/issues)
with the `documentation` label. For a page on the site, the fix belongs in the
`ziradocs/website` repo.
