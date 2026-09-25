# Nested list types validation

Validation ran on `misael@misael-ubuntu.taild699ea.ts.net` in an isolated
checkout of `c2fea7db076d2ddbe9c76eefc194ea667a009684` under
`/mac-remote-development/checkouts/ziradocs-toolchain/ce-t3-c2`.
The Go dependency fingerprint was
`c748f1b46d005585b8f33d9d5bb225df0151e5a726c40867cfc62c00a7ea9355`
(Go 1.27.0, three modules' `go.mod`/`go.sum` files). The TypeScript dependency
fingerprint was
`43478e0acda6eed70a74b5a96b8790d7d671d1a717b253a140b839a269d640ad`
(Node 24.21.0, package manifest and lockfile). Both bundles were populated
once and reused from `/mac-remote-development/dependencies/ziradocs-toolchain/`.

With an ephemeral `go.work` using this checkout's core, each module passed:

```sh
go build -mod=readonly ./...
go vet -mod=readonly ./...
go test -mod=readonly ./... -count=1
```

`ast-types` passed `npm run build` using its locked dependency bundle.
The candidate SlideLang CLI emitted AST 2.16.0 with
`["nested-list-types-v1"]`, preserving the types owned by ParentA, ChildA,
and ParentB. That JSON passed `schema/ast.schema.json`. The candidate DocLang
CLI emitted nested Markdown and DOCX, while SlideLang emitted HTML and PPTX;
the DOCX and PPTX were valid ZIP/XML packages. A legacy filter advertising
only AST 2.14.0 failed the CLI handshake in both languages before receiving
AST bytes. A compatible 2.16.0 filter passed and preserved the list types.

Candidate CLI binaries were built from the exact commit with
`vcs.modified=false`:

| Binary | SHA-256 |
| --- | --- |
| SlideLang | `b7e07453abedfba0ae4e9295a19c130a1821f6578918a98d3f049fcf894c5ca8` |
| DocLang | `d17e3904deadb83e8ab10b3c9e97bc33d7fe908cd528e95b170c60de35abbd21` |

The real published core/v2.37.0 reader accepted candidate 2.16.0 JSON but
discarded `subListType` (`accepted=true`, `subListTypePreserved=false`). With
`GOWORK=off`, DocLang and SlideLang builds fail against that published core
because `PointItem.SubListType` is unavailable. The corresponding published
mode CI jobs remain red until a compatible core release is authorized and
the CLI module dependencies are updated. No release is part of this PR.
