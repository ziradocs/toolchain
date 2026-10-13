# Explicit Markdown content fragments

`content-markdown-v1` is an explicit content-context policy, separate from
historical `typed-headings-v1`. It introduces schema 2.24.0 when used with
actual typed headings. Existing document `#`/`##` slide/subtitle boundaries
remain unchanged. The new source contract permits `SECTION` levels 1–6 in a
strict SLIDE only when both capabilities were explicitly declared.

Use `parser.ParseMarkdownContent(markdown, parser.ContentOptions{AnchorNamespace:
"document/slot/block"})`, then `formatter.FormatContentFragment`, or the common
checked transform `transform.NormalizeMarkdownContent`. The original is never
rewritten. The transform reparses ordinary strict source and requires identical
ordered payloads and identities before accepting output.

The CLI entry is:

```
slidelang fmt original.md --content-fragment --fragment-namespace document/slot/block --fragment-receipt receipt.json --output fragment.strict
```

Output is the indented element body, without frontmatter or SLIDE. Assemble
slides using the caller's outline boundaries and declare the exact capabilities
from each receipt before an ordinary build. Preserve fragment trailing bytes
and use native CODE-aware structural boundaries. Do not rebuild from composed
pages. A candidate binary is not a published product release.

ATX headings 1–6 preserve their authored level and inline Markdown. Generated
anchors include a stable namespace digest and heading ordinal. An explicit
`{#author-anchor}` suffix is preserved verbatim (safe `[a-z0-9_-]` only);
duplicate or invalid anchors reject rather than being renamed. Ordinary strict
assembly rejects duplicate explicit anchors across fragments. Raw HTML never
establishes heading authority.

The existing native registry and strict formatter own paragraphs, lists, tables,
quotes, CODE and embedded syntax. CODE bytes, language, filename, TAB, whitespace
and terminal LF are compared before and after ordinary strict reparse. The public
native `CODE{verbatim}` representation is reused when necessary; no dedent or
trim is applied by this API. A form that cannot survive exactly is rejected and
is not counted as supported. Nested structures that the ordinary strict codec
cannot represent are likewise rejected; this API does not add a private parser.

The sole extra admonition profile is an explicit `> [!WARNING]` marker followed
by one nonempty quoted body line. It produces native warning with the literal
body retained. Multi-line/empty warnings reject; warnings are never inferred
from prose or inside CODE. Document metadata, authored node-id directives,
slide/section markers, CR line endings and unclosed CODE fences are rejected.
The API does not promise full CommonMark coverage (for example setext headings,
raw HTML semantics and thematic breaks).

The external `content-markdown-normalization-v1` receipt contains original
Markdown, input/output SHA256, namespace, exact used schema/capabilities,
diagnostics and ordered node/descendant mappings with identities, input and
output positions, anchors and payload hashes. `accepted` is distinct from
`outputProduced`; validation alone does not claim publication. CLI publication
requires fresh output/receipt paths, never overwrites retained originals and
publishes only a rejected receipt with no source on failed conversion.
