# Code body indentation

Status: syntax only. No change to the JSON AST or to the schema version, and no
capability. Without the attribute every strict source keeps its meaning.

## Problem

A strict `CODE` block takes its body from the lines indented deeper than the
`CODE` line. The indentation that belongs to the structure of the file has to be
removed from them, and the parser has no way to know how much of it there is, so
it removes the whitespace that every non-blank line of the body has in common
(compared as a string: a tab is one character, not four columns).

That works for the usual body. It cannot tell the structural whitespace from
whitespace that belongs to the code when all the lines share some:

```go
	foo()
	bar()
```

A fenced block with this body, written back as a strict `CODE` block, reads back
as `foo()` and `bar()`. The same happens to a body whose first line is the most
indented one, and to a snippet taken from inside a block. Before the attribute
below, `slidelang fmt` and `doclang fmt` refused these bodies by naming the
`code` element.

## `CODE{verbatim}`

An attribute after the keyword, with no space, like `:::card{typed}` in flex:

```slidelang
SLIDE content
  title: "Snippet"
  CODE{verbatim} go
      foo()
      bar()
```

The structural indentation is fixed as that of the `CODE` line plus two spaces,
four spaces here, and everything a line has beyond it is code. The lines above
have six, so the body is `  foo()` and `  bar()`. The language and the filename
follow as usual (`CODE{verbatim} go main.go`, or the bare `CODE{verbatim}`).

The block still ends at the first non-blank line that is not indented deeper than
the `CODE` line. A line that has less than the structural indentation, which only
a hand-written file can have, loses the indentation it has and no more.

## Writing it

`slidelang fmt` and `doclang fmt` write `CODE{verbatim}` only for a body that needs
it: when every non-blank line starts with the same spaces or tabs, or when the
body is made only of whitespace-only lines. Every other body is written as a plain
`CODE`, byte for byte as before, and a second pass writes the same text. Authors
and generators do not have to write the attribute; a flex fence is still the
simplest way to give a code block.

## Compatibility

The attribute is recognized only as the whole word `CODE{verbatim}` followed by a
space or the end of the line. A strict source that never contains it is read
exactly as before, and an older reader that does not know it sees a `CODE` line
with an unexpected first word, so a file that uses it should be read with a
toolchain that supports it.

Not covered by this repository: the documentation site and the editor extension
describe the `CODE` header as well, and need the same one-line note.
