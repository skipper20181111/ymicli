Test dataset for manual verification of capabilities

Overview

This folder contains a compact, manual test dataset to exercise the 28 capability areas. Use it during development and before releases to validate behavior without needing external repos. It is designed to run locally and be skimmable.

Contents

- `capabilities/cases.yaml`: Master list of test cases (one per capability), with preconditions, steps, and expected results.
- `capabilities/prompts/`: Reusable prompts for agentic/reasoning features.
- `projects/sample-go/`: Small Go project used by file/search/edit tools and LSP.
- `context/CRUSH.md`: Example project memory/context file.
- `context/.crushignore`: Ignore rules to validate search tools behavior.

How to use

1) Build the binary (from repo root):
   - go build -o ./bin/crush ./

2) Open the TUI from repo root:
   - ./bin/crush

3) Create a new session and follow the relevant test case in `capabilities/cases.yaml`.

Provider/API setup notes

- Some agentic/reasoning tests require valid provider configuration and API keys. Use the onboarding splash to pick a model and set a key.
- For features not requiring LLM calls (file ops, search, LS, permissions), you can test them without keys.
- MCP tests require a reachable MCP server (stdio/HTTP/SSE). Use `mcp-tools` examples from the server you have available.

Git tests

- Initialize a local git repo in `projects/sample-go` to validate git-only commands in Bash:
  - cd testdata/projects/sample-go
  - git init && git add . && git commit -m "init"

Safety

- The Bash tool blocks many commands for safety and uses explicit permissions. Expect permission prompts for write/exec operations.

