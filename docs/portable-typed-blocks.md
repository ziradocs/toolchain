# Typed special blocks

Status: implemented in #414. Opt-in per block, no change to the JSON
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

A block opts in by a `typed` attribute on its opening line, in strict only. The
block syntax already has an attribute list (`:::card{type="success"}`), and the
marker lives there, so it works with a title too
(`:::details{typed} Advanced settings`) and never meets one:

```
:::card{typed}
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
no new element: the AST of a strict `:::card{typed}` equals the AST of the flex
`:::card` with that body.

| Dialect | Block (unchanged) | Typed block (new) |
| --- | --- | --- |
| strict | `:::card` | `:::card{typed}` |
| flex | `:::card` | `:::card{typed}` is not special: `{typed}` stays part of the block type |

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

## Which blocks

The marker is read on the opening line of any strict `:::` block, because they
all go through one parser (`SpecialBlockParser`). It matters for the blocks whose
body the flex reading turns into nested elements. Those are:

- `card`, with its variants (`type="success"`, `type="warning"`),
- `columns` (with `ratio` and `count`),
- `tabs` (the `::tab{title=...}` lines stay prose inside it, as in flex today),
- `accordion`,
- `details` and `reveal`,
- the callouts the parser knows by name: `info`, `warning`, `danger`, `success`,
  `tip`, `note`, `example`, `left`, `right` and `highlight`.

That list is what the 71 flex examples and the parser's `KnownSpecialBlockTypes`
contain. A block of any other type that a deck invents gets the same behaviour,
since nothing in the reading depends on the type; the spec lists the types above
as the ones with a rendering.

## No change of meaning

A first draft put the marker as the last word of the line, which would have changed
the meaning of a strict title ending in `typed` ("Dynamically typed") whenever its
body had nested elements, with no warning. Putting it in the attribute list removes
that: it never meets a title, no existing source reads differently, and no
diagnostic is needed. The reader stays as it is for every other attribute (the
attribute text remains in the block type); only a whole word `typed` outside quotes
in a strict block's braces is the flag, and the block type is what is left, so
`:::card{type="success" typed}` has the block type `card{type="success"}`, the
same as the flex block. `fmt` refuses a flex block whose type is exactly the text
`...{typed}`, since writing it would read back as the marker.

## Formatter

`formatSpecialBlock` writes the `typed` attribute on the opening line when the block has
nested elements and strict would not reproduce them. It reads the candidate text
back (inside a one-slide document) and keeps the marker only if the title, the
content and the nested elements then read back equal. The typed reading removes the
block's own indentation from the lines first, so a fence inside a block sees the
same lines it would in flex. A block whose elements strict already
reproduces, and every prose-only block, formats byte for byte as it does today.
When even the typed reading does not reproduce them, the refusal naming
`special_block` stays.

`doclang fmt --strict` shares the formatter and the parser, so documents get the
same form.

## AST and JSON

No change. `SpecialBlockElement.elements` and `content` are already in the
schema. The version stays as it is.

## Compatibility

Existing sources read as before: a strict block without `typed` in its attribute
list is untouched, and so is every flex block. The change is in the strict
special-block parser, which
lives in `core`, so the release follows the repository sequence: `core/vX.Y.Z`
first, then the pin bump in `slidelang/go.mod` and `doclang/go.mod`.

## Spec changes

In "Strict Mode Grammar", under `special_block`:

```ebnf
special_block ::= ":::" block_type ("{" attributes "}")? title? NEWLINE block_content ":::"
```

and a section, "Typed special blocks" (in the spec, as built):

- A `typed` attribute on the opening line makes the body read with the flex
  nested-content recognizers (headings, fenced code, pipe tables, images,
  embedded charts and nested blocks), after removing the block's own indentation,
  so `elements` is filled the way a flex block fills it; without it a strict body
  is raw text plus the elements strict itself recognizes.
- **Two views, not one.** In a block `:::` both `content` (the trimmed raw lines)
  and `elements` are filled, and the renderer prefers `elements` when it is not
  empty. In a typed grid column (`::: column typed`, `<<column typed>>`) only
  `elements` is filled and `content` is empty. The difference is deliberate: a flex
  block has always filled both, so the strict form has to reproduce both to build
  the same AST, while a flex column never had `elements`, so its typed form is a new
  shape. A column also has no title, so a trailing word is unambiguous there, while
  a block's marker has to stay clear of the title and lives in the attribute list.
- The blocks that carry nested elements: `card` (and its `type` variants),
  `columns`, `tabs`, `accordion`, `details`, `reveal`, `info`, `warning`,
  `danger`, `success`, `tip`, `note`, `example`, `left`, `right`, `highlight`.
- Flex is unchanged: there `{typed}` stays part of the block type like any other
  attribute.
- `<!-- node-id -->` inside a typed block binds to the nested node that starts
  on the next line, identically in both dialects (second stage).

The word `typed` was kept for consistency with `::: column typed`.

## Node identities (after the first version, designed to be additive)

`<!-- node-id: Name -->` already binds inside a block body wherever a nested
element is recognized: the identity pre-pass removes the comment line before the
body is read, so `content` never contains it, and binds the name to the node
that starts on the next line. A prose run starts on its first line, so an
identity before it binds to that `TextElement`. Typed blocks keep that rule, and
because the pre-pass runs before either dialect reads the body, a marker inside
a typed block binds to the same node in flex and in strict. Nothing is added to
make that true; what the proposal adds is the test.

The first version ships without promising it (the opt-in is about elements), and
the second adds the AST-equivalence test for identities: the same card written in
flex and in strict (`:::card{typed}`) with `node-id` comments before its children
has the same tree including `nodeId`, and `fmt` places the comments back on the
nested lines by the same read-back that `formatNodeIDs` uses for blocks today.
Because binding is by position and the comment lines never reach `content`,
adding it later changes no source and no AST that exists before it.

## Out of scope

A heading recognizer for strict blocks that does not need the marker.

## What stays refused

A block's `content` is trimmed line by line, while a nested fenced code block keeps
the indentation it had. Strict text only has the trimmed lines, so a typed block
reads such code back without its indentation, and the formatter keeps refusing the
block naming `special_block` (two example decks). A fence whose info string has more
than one word after the language (```` ```python {1,3-5} ````) keeps the whole
string as the language, and strict reads the second word of a `CODE` line as a file
name, so the formatter refuses it naming `code`.
