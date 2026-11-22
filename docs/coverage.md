# Go test coverage

The `go-quality` job runs the full Go test suite with a coverage profile and writes total statement coverage to the
GitHub Actions job summary. This keeps the measurement visible on every pull request and main-branch push.

At the 2026-11-23 checkpoint after PR #484, the full-module statement coverage is **91.4%** (rounded to one
decimal place). Package coverage is buffer 99.1%, concurrency 92.9%, file 85.3%, log 78.2%, metadata 78.1%, query
100.0%, record 96.7%, recovery 97.4%, storage 100.0%, and transaction 90.8%. Recent focused tests closed existing
`Expression.Evaluate`, `Page.GetBytes`, `Page.SetBytes`, and metadata nil-schema, integer-range, scan-open, scan-close,
and both catalog-initialization failure paths. The remaining uncovered statements are being inspected by package and
behavior rather than hidden behind a premature gate.

The CI step reports coverage without enforcing a threshold while the repository is below 100%. Add a required 100%
threshold only after the complete suite measures 100.0%; otherwise CI would be permanently red before the target is met.

For a local full-module measurement, run:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

Inspect individual uncovered functions with:

```sh
go tool cover -func=coverage.out
```
