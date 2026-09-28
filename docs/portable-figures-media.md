# Figures and media without silent loss

These changes close several places where figure content was accepted but
silently dropped, moved, or invented.

## Diagram captions

`<<mermaid>>` and `<<plantuml>>` accept a caption on the opening tag, in flex
and strict:

```slidelang
<<mermaid title="Approval flow">>
  graph TD
    A --> B
<<end>>
```

The caption fills the existing `title` field of `MermaidElement` and
`PlantUMLElement`, which the renderer already draws. The AST schema does not
change. Single quotes are accepted when the caption contains double quotes.
Any other attribute, a repeated or empty `title`, or malformed quoting is an
error; previously such a tag fell through to plain text. `slidelang fmt` and
`doclang fmt` write the caption back on the tag. Charts keep their existing
`title:` property.

## Media poster and caption (schema 2.18.0)

`<<video>>` and `<<audio>>` accept `poster=` (video only) and `caption=`:

```slidelang
<<video src="demo.mp4" poster="poster.jpg" caption="Product demo" controls>>
```

`MediaElement` gains `poster`, `caption`, and `captionHTML`. Like
`table-rows-v1`, the capability `media-figure-v1` is inferred from the authored
attributes and is not declared in frontmatter. A document that uses it emits
schema 2.18.0, alongside any other capability it uses; a media element without
poster or caption keeps the legacy contract. The decoder and the JSON Schema
reject poster or caption without the capability and the capability without
them.

HTML renders the poster (same URL policy as `src`; a blocked poster is
dropped, the video stays playable) and wraps a captioned element in
`<figure>`/`<figcaption>`. DocLang Markdown and DOCX add the caption under the
media link. Any other attribute on the tag now produces warning `MEDIA001`
instead of disappearing.

## Chart `null` values

In chart `data`, `null` is the missing-value marker of Chart.js and is now
kept in its column. Previously it was dropped, so `["B", null, 20]` became
`["B", 20]` and `20` moved to the wrong series. The formatter writes `null`
back, the browser chart receives it in place, and the offline PNG renderer
(offline HTML and PPTX) draws it as a gap. No schema change.

## Map markers without coordinates

A map marker without usable coordinates is now an error in every syntax:
`marker:` with fewer than two fields or non-numeric values, a `- lat:` item
without `lng:`, and a JSON marker with neither `position` nor both `lat` and
`lng`. Previously such markers were placed at 0,0 or silently dropped. An
explicit 0 remains a valid coordinate. This is a behavior change for documents
that relied on the old default.

## Node IDs after diagrams

A `<!-- node-id: Name -->` annotation right after a `<<mermaid>>`,
`<<plantuml>>`, `<<chart:…>>`, `<<map>>`, or `<<math>>` block that ends by
indentation (without `<<end>>`) now binds to the next element. Previously the
identity pre-pass waited for `<<end>>` and reported the annotation as an orphan.
