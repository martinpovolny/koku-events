# koku-events agent instructions

## Serena semantic tooling

- For coding tasks in this repository, activate the registered Serena project `cost_ai_grid_poc` before exploring or editing Go code.
- Prefer Serena's Go language-server tools for symbol discovery, references, diagnostics, and refactoring. Use semantic search before broad text search when the target is a Go symbol.
- Read Serena's `initial_instructions` once per session before using its project tools. Use the project memories when they are relevant.
- Use shell tools for builds, tests, formatting, Git operations, and non-code files. If Serena is unavailable or its Go language server fails, report the exact failure and its impact rather than silently substituting a different semantic tool.

## Project entrypoints

- The Go module and service are under `inventory-watcher/`.
- The main consumer entrypoint is `inventory-watcher/cmd/consumer/main.go`.
- Run the affected tests and checks described in `inventory-watcher/Makefile` and `CLAUDE.md` before completing code changes.
