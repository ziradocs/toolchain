# Explicit Markdown content fragments

`content-markdown-v1` is an explicit content-context policy, separate from
historical `typed-headings-v1`. It introduces schema 2.24.0 when used with
actual typed headings. Existing document `#`/`##` slide/subtitle boundaries
remain unchanged. The new source contract permits `SECTION` levels 1–6 in a
strict SLIDE only when both capabilities were explicitly declared.

Use `parser.ParseMarkdownContent(markdown, parser.ContentOptions{AnchorNamespace:
"document/slot/block"})`, then `formatter.FormatContentFragment`, or the common
checked transform `fragment.NormalizeMarkdownContent`. The original is never
rewritten. The transform reparses ordinary strict source and requires identical
ordered payloads and identities before accepting output.

The CLI entry is:

```
slidelang fmt original.md --content-fragment --fragment-namespace document/slot/block --fragment-receipt receipt.json --output fragment.strict
```

Output is the indented element body, without frontmatter or SLIDE. Assemble
slides using the caller's outline boundaries and declare `receipt.sourceCapabilities`
before an ordinary build. `receipt.capabilities` lists the derived AST extensions;
these are distinct from source opt-in declarations. Preserve fragment trailing bytes
and use native CODE-aware structural boundaries. Do not rebuild from composed
pages. A candidate binary is not a published product release.

ATX headings 1–6 preserve their authored level and inline Markdown. Generated
anchors include a stable namespace digest and heading ordinal. An explicit
`{#author-anchor}` suffix is preserved verbatim (safe `[a-z0-9_-]` only);
duplicate or invalid anchors reject rather than being renamed. Ordinary strict
assembly rejects duplicate explicit anchors across fragments. Local links must
resolve to preserved explicit anchors in the same fragment; unresolved links
to old auto-derived anchors are diagnosed, not silently broken. This profile
does not infer navigation between fragments. Raw HTML never
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
standalone slide/section marker lines in structural positions, indented ATX (potential literal code), CR line endings and unclosed CODE fences are rejected.
The API does not promise full CommonMark coverage (for example setext headings,
raw HTML semantics and thematic breaks).

In content context only, one empty blockquote line immediately before a final
`— author` or `-- author` line is an attribution separator rather than a body
LF. Internal paragraph separators and quotes without attribution retain their
body lines; direct attribution needs no separator. Multiple empty lines before
attribution are ambiguous and reject without output. Historical flex document
parsing retains its existing behavior. No generic body trimming is performed. Ten unchanged Writer/source witnesses from
e32 (A081/A091/A096) are recorded in `core/fragment/testdata/quote-attribution-e32.json`
and checked through fragment normalization and ordinary strict reparsing.

The external `content-markdown-normalization-v1` receipt contains original
Markdown, input/output SHA256, namespace, exact used schema/capabilities and sourceCapabilities declarations,
diagnostics and ordered node/descendant mappings with identities, input and
output positions, anchors and payload hashes. `accepted` is distinct from
`outputProduced`; validation alone does not claim publication. CLI publication
requires fresh output/receipt paths, never overwrites retained originals and
publishes only a rejected receipt with no source on failed conversion.
