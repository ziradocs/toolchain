# Code block filenames (schema 2.19.0)

A code block can carry the name of the file it shows. Previously the name was
lost: strict `CODE typescript renewals.ts` kept only the language, and a flex
fence ```` ```typescript renewals.ts ```` put the whole line in `language`.

## Authoring

Strict, in SlideLang or DocLang:

```slidelang
CODE typescript renewals.ts
  export const renewals = [];
```

Flex, with the filename after the language or as `title="…"`:

````markdown
```typescript renewals.ts
export const renewals = [];
```
````

The filename is a single token and requires a language before it. More tokens
after the filename are an error. Fence info that starts with `[` (code-group
labels) or `{` (highlighted line ranges) is not a filename and keeps its
previous meaning.

## AST and versions

`CodeElement` gains `filename`. The capability `code-filename-v1` is inferred
from that syntax, like `table-rows-v1` and `media-figure-v1`, and a document
that uses it emits schema 2.19.0 alongside any other capability it uses. The
decoder and the JSON Schema reject a filename without the capability, the
capability without a filename, and a filename that is not a single token.

## Output

HTML wraps a named block in `<figure>` with the name as `<figcaption>`;
SlideLang slides show it above the code; PPTX adds a first line with the name;
DocLang Markdown puts it in backticks above the fence; DOCX adds it in bold
above the code. `fmt` writes it back in both dialects.
