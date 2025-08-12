# CRUSH Context (Test Dataset)

Build
- go build ./...
- go test ./...

Style
- Descriptive naming; avoid 1–2 char identifiers
- Early returns; handle errors first
- Minimal comments for complex logic (why > how)

Repo
- Minimal sample project + capability tests
- Prefer absolute paths in tools

Testing
- Use testdata/projects/sample-go
- Avoid find/grep/cat in Bash; use Glob/Grep/View tools

Conventions
- Respect .crushignore in search tools
- Keep edits compilable; check diagnostics after edits