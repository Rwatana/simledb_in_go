# Go test coverage

The `go-quality` job runs the full Go test suite with a coverage profile and writes total statement coverage to the
GitHub Actions job summary. This keeps the measurement visible on every pull request and main-branch push.

At the 2026-11-22 Chapter 8 predicate checkpoint, the full-module baseline was **90.7%**. The CI step reports coverage
without enforcing a threshold while the repository is below 100%. The uncovered statements are being audited and tested
in focused changes. Add a required 100% threshold only after the complete suite measures 100.0%; otherwise CI would be
permanently red before the target is met.

For a local full-module measurement, run:

```sh
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

Inspect individual uncovered functions with:

```sh
go tool cover -func=coverage.out
```
