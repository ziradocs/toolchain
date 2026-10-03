# Typed special blocks

Status: proposal, not implemented. Opt-in per block, no change to the JSON
schema. Without the opt-in, every existing source and AST keeps its current
meaning byte for byte.

## Problem

`slidelang fmt` and `doclang fmt --strict` cannot transpile a flex deck whose
`:::card`, `:::columns`, `:::tabs` or `:::accordion` holds a heading, fenced
code, a Markdown table or an image. They refuse it, naming `special_block`,
because strict has no text that builds the same AST. Cards and columns are
common in real decks, so the refusal is a real limit (five of the 71 flex
examples hit it).

The cause is in how a block body is read. `SpecialBlockElement` carries two
parallel views of one body:

- `content`: the raw lines of the body, trimmed.
- `elements`: the lines that a nested-content recognizer claimed (a
  `###` heading, a fenced code block, a pipe table, `![alt](src)`, a chart, a
  nested block), with the loose prose between them as `TextElement`s. It is
  filled only when at least one line was claimed.

The recognizer set depends on the dialect. Flex recognizes `###`, fences and
`![](..)`; strict recognizes `|` tables and `<<chart>>`, but its own spellings
of the rest (`CODE`, `IMAGE`) need indentation and keywords that the trimmed
body does not keep, and it has no heading recognizer inside a block at all. The
formatter writes `content`, so the strict text reads back with `elements`
empty. Nothing in strict says "read this body the way flex does".

## What typed columns settled, and what does not carry over

`<<column typed>>` (#398) gives a grid column a typed body: `content` stays
empty and `elements` holds the children. That works because a raw flex column
has no `elements` either, so a typed column is a new shape, not a different
reading of an existing one.

A special block is not like that. A flex `:::card` already fills both `content`
and `elements`, and the AST the build produces for it contains both. A strict
form that left `content` empty would build a different AST from the flex source
it came from, which is the property fmt promises. So for blocks the marker
cannot mean "children only"; it has to reproduce the two views that flex
produces.

## Decision

A block opts in by a trailing word on its opening line, in strict only:

```
:::card typed
### Improved performance
Faster builds.
- one
- two
:::
```

`typed` means: read the body with the same nested-content recognizers a flex
block uses. The body is still the trimmed lines up to the closing `:::`, so
`content` is exactly what it is today, and `elements` is exactly what flex
produces for the same lines, because it is the same code path. No new field and
no new element: the AST of a strict `:::card typed` equals the AST of the flex
`:::card` with that body.

| Dialect | Block (unchanged) | Typed block (new) |
| --- | --- | --- |
| strict | `:::card` | `:::card typed` |
| flex | `:::card` | `:::card typed` is not special: `typed` stays part of the title |

Flex needs no marker because it already reads every block that way. Leaving it
alone also means no flex source changes meaning.

Why a marker and not the alternatives:

- **Reading every strict block with the flex recognizers** would change the AST
  of any strict source whose block body has a line starting with `###`, a
  fence, or `![`. That is not additive.
- **A document-level opt-in** (`ast_capabilities`) switches every block at once
  and needs frontmatter handling in `fmt`, for a change that is local to one
  block.
- **Dropping `content` for typed blocks** (the shape of typed columns) cannot
  equal what flex builds today, as above.

The one source that changes meaning is a strict block whose title is exactly
the word `typed` (`:::info typed`): the word becomes the marker and the block
loses that title. Any other title is untouched. `fmt` refuses a flex block whose
title is exactly `typed` and has nested elements, since writing it would read
back as the marker.

## Formatter

`formatSpecialBlock` writes ` typed` on the opening line when the block has
nested elements and strict would not reproduce them. The check already exists
(`checkNestedBlockElements` reads the text back and compares the nested
elements); the change is to retry with the marker and keep it only if the
elements then read back equal. A block whose elements strict already
reproduces, and every prose-only block, formats byte for byte as it does today.
When even the typed reading does not reproduce them, the refusal naming
`special_block` stays.

`doclang fmt --strict` shares the formatter and the parser, so documents get the
same form.

## AST and JSON

No change. `SpecialBlockElement.elements` and `content` are already in the
schema. The version stays as it is.

## Compatibility

Existing sources read as before, with the one exception above (a strict title
that is exactly `typed`). The change is in the strict special-block parser, which
lives in `core`, so the release follows the repository sequence: `core/vX.Y.Z`
first, then the pin bump in `slidelang/go.mod` and `doclang/go.mod`.

## Spec changes to apply when approved

In "Strict Mode Grammar", under `special_block`:

```ebnf
special_block ::= ":::" block_type ("typed")? NEWLINE block_content ":::"
```

and a paragraph: a trailing `typed` on the opening line makes the body read
with the flex nested-content recognizers (headings, fenced code, pipe tables,
images), so `elements` is filled the way a flex block fills it; without it a
strict body is raw text plus the elements strict itself recognizes.

## Open questions

- The word. `typed` matches `<<column typed>>`, but here it selects a reading
  rather than a shape, so `flex` or `nested` would describe it better. I would
  keep `typed` for consistency unless you prefer otherwise.
- Node identities. A `<!-- node-id -->` inside a block body is not bound today
  (the nested lines are trimmed and re-read), and this does not change it.
- Scope. The marker applies to every `:::` block, not only the four named,
  because they share one parser.

## Out of scope

Giving block children node identities, and a heading recognizer for strict
blocks that does not need the marker.
