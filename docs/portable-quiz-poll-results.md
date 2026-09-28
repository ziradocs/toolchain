# Quiz and poll results (schema 2.20.0)

A quiz or poll can carry the results it collected: one percentage per option
and, optionally, the number of responses.

## Authoring

The capability `quiz-poll-results-v1` is inferred from the new `results:` and
`responses:` keys; no frontmatter declaration is needed, and declaring it in
`ast_capabilities` is an error, as for `table-rows-v1`:

```slidelang
<<poll>>
  question: "Preferred plan?"
  options: ["Basic", "Pro", "Team"]
  results: [20, 45.5, 34.5]
  responses: 120
<<end>>
```

`results` is parallel to `options`: one number from 0 to 100 per option. The
values do not have to add up to 100, since rounding, multiple-choice polls and
partial results make other sums legitimate. `responses` is an optional
non-negative integer. The same keys work in `<<quiz>>`.

## Validation

The linter reports `QUIZ003`/`POLL003` when `results` does not have one value
per option, when a value is outside 0–100, or when `responses` is negative.
The decoder and the JSON Schema also enforce the ranges and reject the fields
without the capability or the capability without the fields. Before this
change the keys were reported as unknown and dropped.

## AST, versions and output

`QuizElement` and `PollElement` gain `results` and `responses`. A document
that uses them emits schema 2.20.0 with `quiz-poll-results-v1`, alongside any
other capability it uses. HTML and slides show the percentage next to each
option and the response count below the list; DocLang Markdown and DOCX and
SlideLang PPTX append the percentage to each option line and add the count.
`fmt` writes both keys back.
