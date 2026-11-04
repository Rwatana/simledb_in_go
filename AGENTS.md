# Repository Instructions

## Workflow

- Start every change from a GitHub Issue with explicit acceptance criteria.
- Use `feature/#<issue-number>_<snake_case_description>` for branch names.
- Use the GitHub Issue number that defines the change; do not omit `#` or `/`.
- Prefix Issue and Pull Request titles with an allowed value from `hack/prefix.yml`.
- Put `## Issue Number` and the matching `#<issue-number>` in every Pull Request body.
- Keep exactly one commit in each Pull Request. Amend that commit for revisions.
- Add `Closes #<issue-number>` to the Pull Request body, using the same number as the branch.
- Keep each Pull Request focused on one learning outcome or observable behavior.

## Implementation

- Follow the dependency rule documented in `docs/architecture.md`.
- Do not reproduce the Java class hierarchy mechanically.
- Introduce a package or interface only when the current behavior needs that boundary.
- Write a failing test before implementation code.
- Use temporary directories for tests that touch the filesystem.
- Run `gofmt`, `go test ./...`, and `go vet ./...` for Go changes.

## Documentation

- Give every exported Go name a Go doc comment.
- Start doc comments with the declared name and write complete sentences.
- Update the relevant chapter note with the Java source map, Go design decision,
  tests, and intentional differences.
- Keep diagrams editable as `.drawio` files and explain their meaning in Markdown.
- Do not commit the book PDF or the upstream Java distribution.

## Validation

- Run `python3 scripts/check_docs.py` for every change.
- Do not bypass the branch-name or one-commit Pull Request CI checks.
