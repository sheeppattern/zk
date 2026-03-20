# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
# Build binary
go build -o zk.exe .

# Run all tests
go test ./... -race -cover

# Run specific package tests
go test ./internal/model/ -v
go test ./internal/store/ -v
go test ./cmd/ -v -count=1    # integration tests (builds binary)

# Build with version + skill version (git hash)
go build -ldflags "-X github.com/sheeppattern/zk/cmd.Version=0.1.0 -X github.com/sheeppattern/zk/cmd.SkillVersion=$(git rev-parse --short HEAD)" -o zk.exe .
```

## Skill Content Sync Policy

When CLI commands, flags, relation types, or workflows change, update the skill instruction content in `cmd/skill_cmd.go`:
- `zkInstructionContent` — shared command reference used by all 6 agent tools (Claude, Gemini, Codex, Cursor, Copilot, Windsurf)
- `domainGuideContent` — best practices and domain knowledge
- These are the primary way AI agents learn to use zk, so they must stay accurate and complete
- After updating, run `zk skill generate --project-dir .` to verify the generated files render correctly

## Test Coverage Policy

When adding or modifying features, always write tests:
- `internal/` packages: unit tests in `*_test.go` using `t.TempDir()` for filesystem isolation
- `cmd/`: integration tests in `cmd/integration_test.go` using `runZK()` helper via `exec.Command`
- Run `go test ./... -cover` before committing and verify coverage does not decrease
- New features in store.go require round-trip serialization tests (create → save → load → verify)
- New CLI commands require at least one E2E test via `runZK()`

## Architecture

Zettelkasten memory CLI for AI agents. Single binary Go app using cobra.

### Layers

```
main.go → cmd/ → internal/store/ → filesystem
                → internal/model/    (data structs)
                → internal/output/   (JSON/YAML/MD rendering)
```

- **cmd/**: Each file is one cobra command group. All commands share global state via `flagProject`, `flagFormat`, `flagVerbose`, `flagQuiet` defined in `root.go`. Use `statusf()` for stderr status, `debugf()` for verbose output.
- **internal/model/**: Note, Link, Project, Config structs. `Note.Content` is tagged `yaml:"-" json:"-"` — stored in Markdown body, not frontmatter. `NoteFrontmatter` is the serializable mirror. `Note.Layer` distinguishes "concrete" (default) from "abstract".
- **internal/store/**: File I/O layer. Notes are `.md` files with YAML frontmatter. Frontmatter parsing is manual string splitting. Uses `atomicWriteFile` for crash-safe writes. **When adding new Note fields, update both `marshalNote` and `unmarshalNote` to copy the field to/from `NoteFrontmatter`.**
- **internal/output/**: Formatter dispatches to JSON/YAML/MD renderers. Uses `noteView` wrapper to re-include Content in output.

### Key Patterns

**Store path resolution** (`getStorePath` in root.go): `--path` flag → `ZKMEMORY_PATH` env → `~/.zk-memory`

**Project scoping**: Empty `flagProject` = global (`global/notes/`), non-empty = `projects/{id}/notes/`.

**Bidirectional links**: `link add` writes Link on BOTH notes. Backlink queries scan all notes O(n). Cross-project via `--target-project`.

**Note layers**: `concrete` (facts/records) vs `abstract` (insights/questions). `zk reflect` analyzes concrete notes and suggests abstract ones.

**Output contract**: stdout = pure data, stderr = status/errors. Use `statusf()`/`debugf()`, never raw `fmt.Fprintf(os.Stderr, ...)`.

**Multi-agent skills**: `zk init` generates instruction files for 6 AI tools (Claude, Gemini, Codex, Cursor, Copilot, Windsurf). Content is shared; only frontmatter wrappers differ per tool.

### Storage Layout

```
{store_path}/
├── config.yaml
├── projects/{project-id}/
│   ├── project.yaml
│   └── notes/{note-id}.md
├── global/notes/{note-id}.md
├── trash/                      # soft-deleted notes
└── templates/                  # note templates (.yaml)
```

### ID Generation

`model.GenerateID(prefix)` produces IDs like `N-72F576` (prefix + 6 uppercase hex chars from UUID). Prefixes: `N-` for notes, `P-` for projects.

### Module Path

`github.com/sheeppattern/zk` — all internal imports use this prefix.
