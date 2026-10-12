# Literal payloads and ordered list starts

The legacy AST remains 2.14 when no extension is used. CODE and CODE_GROUP
preserve empty first payload lines in both dialects without an opt-in. A group
has its own node identity and ordered language/label/content tabs; tabs have no
independent node IDs.

`ast_capabilities: [list-start-v1]` opts source into AST 2.21. An ordered
POINTS list emits `start`, including 1. It is an optional positive JSON safe
integer (1 through 9007199254740991); unordered lists omit it. A nested ordered
list stores `subListStart` on the owning item, requires nonempty `subPoints`
and `subListType: ordered`, and requires `nested-list-types-v1`. Source ordinals
must be decimal and contiguous. Invalid, alphabetic or discontinuous ordinals
produce errors. The runtime also rejects a list whose last implied ordinal
(start + item count - 1) exceeds the safe range; the schema constrains the stored
start value and cannot express this arithmetic invariant. Without the opt-in,
existing source list behavior is unchanged.

`ast_capabilities: [math-source-v1]` opts into AST 2.22. The capability is a
whole-document policy: all MathElement content is literal LaTeX. It introduces
no new Math JSON field. It applies equally to source, decoded JSON and ASTs
constructed through the Go API. Blank lines, relative indentation and trailing
spaces are preserved. In Strict, the exact whitespace prefix of the opening
`<<math>>` plus two spaces is structural and is removed from every payload line
that has that prefix. Shorter blank lines are retained; nonblank lines missing
the structural prefix produce errors. Flex removes no payload whitespace.
Opening/closing delimiter lines are structural and are excluded from payload;
inline dollar syntax retains the literal bytes between the two `$$` delimiters.
Angle metadata at payload column zero (`caption:` and `label:` after structural
dedent) remains metadata. The legacy parser continues trimming lines and
omitting blank angle-body lines.

Formatters reject Math payloads that collide with delimiters/metadata and legacy
Math containing whitespace the legacy parser cannot preserve. They also reject
code-group tab labels with leading/trailing whitespace or line breaks and tab
payloads containing a closing fence. A successful formatter result must not hide
payload loss. JSON version/capability gates and the generated TypeScript/schema
agree. Filters negotiate these capabilities before receiving JSON and cannot
remove literal Math policy or change list starts/numbered-list identity ownership.
