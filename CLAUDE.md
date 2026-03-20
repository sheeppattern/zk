# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build binary
go build -o zk.exe .

# Run directly
go run .

# Build all packages (compile check)
go build ./...

# Install dependencies
go mod tidy
```

No test suite exists yet. No linter is configured.

## Architecture

Zettelkasten memory CLI for AI agents. Single binary Go app using cobra.

### Layers

```
main.go → cmd/ → internal/store/ → filesystem
                → internal/model/    (data structs)
                → internal/output/   (JSON/YAML/MD rendering)
```

- **cmd/**: Each file is one cobra command group. All commands share global state via `flagProject`, `flagFormat`, `flagVerbose` defined in `root.go`.
- **internal/model/**: Note, Link, Project, Config structs. `Note.Content` is tagged `yaml:"-" json:"-"` — it's stored in the Markdown body, not in YAML frontmatter. `NoteFrontmatter` is the serializable mirror of Note without Content.
- **internal/store/**: File I/O layer. Notes are `.md` files with YAML frontmatter (`---\nYAML\n---\nContent`). Frontmatter parsing is manual string splitting, no external library.
- **internal/output/**: Formatter dispatches to JSON/YAML/MD renderers. Uses `noteView` wrapper structs to re-include Content in output since the model excludes it from serialization tags.

### Key Patterns

**Store path resolution** (`getStorePath` in root.go): `--path` flag → `ZKMEMORY_PATH` env → `~/.zk-memory`

**Project scoping**: Empty `flagProject` string = global (`global/notes/`), non-empty = `projects/{id}/notes/`. All note operations pass `flagProject` to store methods.

**Bidirectional links**: `link add` writes a Link entry on BOTH source and target notes. No reference table — backlink queries scan all notes in the project (O(n)).

**Cross-project links**: `link add` accepts `--target-project` flag. Source note uses `--project`, target uses `--target-project`.

**Output contract**: stdout = pure data (JSON/YAML/MD), stderr = status/error messages. This is critical for AI agent piping.

### Storage Layout

```
{store_path}/
├── config.yaml
├── projects/{project-id}/
│   ├── project.yaml
│   └── notes/{note-id}.md
└── global/notes/{note-id}.md
```

### ID Generation

`model.GenerateID(prefix)` produces IDs like `N-72F576` (prefix + 6 uppercase hex chars from UUID). Prefixes: `N-` for notes, `P-` for projects.

### Module Path

`github.com/sheeppattern/zk` — all internal imports use this prefix.
