# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git Workflow

**모든 작업은 반드시 feature 브랜치에서 수행한다. main에 직접 커밋/푸시 금지.**

```bash
# 1. 브랜치 생성
git checkout -b feature/<작업명>

# 2. 작업 + 커밋 (체크포인트마다)
git add <files>
git commit -m "feat/fix/refactor: ..."

# 3. PR 생성 → 리뷰 → 머지
gh pr create --title "..." --body "..."
gh pr merge <번호> --merge --delete-branch
```

- main 브랜치에 직접 push하지 않는다
- 모든 변경은 PR을 통해 머지한다
- 커밋은 체크포인트마다 자주 한다
- 커밋 메시지: `feat:`, `fix:`, `refactor:`, `docs:` prefix 사용

## Build & Test

```bash
# Build binary
go build -o nete.exe .

# Run all tests
go test ./... -race -cover

# Run specific package tests
go test ./internal/model/ -v
go test ./internal/store/ -v
go test ./cmd/ -v -count=1    # integration tests (builds binary)

# Build with version + skill version (git hash)
go build -ldflags "-X github.com/sheeppattern/nete/cmd.Version=0.1.0 -X github.com/sheeppattern/nete/cmd.SkillVersion=$(git rev-parse --short HEAD)" -o nete.exe .
```

## Skill Content Sync Policy

When CLI commands, flags, relation types, or workflows change, update the skill instruction content in `cmd/skill_cmd.go`:
- `neteInstructionContent` — shared command reference used by all 6 agent tools (Claude, Gemini, Codex, Cursor, Copilot, Windsurf)
- `domainGuideContent` — best practices and domain knowledge
- These are the primary way AI agents learn to use nete, so they must stay accurate and complete
- After updating, run `nete skill generate --project-dir .` to verify the generated files render correctly

## Test Coverage Policy

When adding or modifying features, always write tests:
- `internal/` packages: unit tests in `*_test.go` using `t.TempDir()` for filesystem isolation
- `cmd/`: integration tests in `cmd/integration_test.go` using `runZK()` helper via `exec.Command`
- Run `go test ./... -cover` before committing and verify coverage does not decrease
- New features in store.go require round-trip serialization tests (create → save → load → verify)
- New CLI commands require at least one E2E test via `runZK()`

## Architecture

Zettelkasten memory CLI for AI agents. Single binary Go app using cobra + SQLite.

### Layers

```
main.go → cmd/ → internal/store/ → SQLite (store.db)
                → internal/model/    (Memo, Note, Link, Config)
                → internal/output/   (JSON/YAML/MD rendering)
```

- **cmd/**: Each file is one cobra command group. Global state: `flagNote` (int64), `flagFormat`, `flagVerbose`, `flagQuiet` in `root.go`. `openStore(cmd)` opens DB + calls Init(). Use `statusf()`/`debugf()` for stderr.
- **internal/model/**: `Memo` (atomic record), `Note` (container), `Link` (source_id→target_id), `Config`. All IDs are `int64` autoincrement. No YAML tags (JSON only).
- **internal/store/**: SQLite layer via `modernc.org/sqlite` (pure Go, no CGO). FTS5 for full-text search. WAL mode, `SetMaxOpenConns(1)`. Links stored once (no bidirectional duplication).
- **internal/output/**: Formatter dispatches to JSON/YAML/MD renderers. Uses `memoView` wrapper.

### Key Patterns

**Store path resolution** (`getStorePath` in root.go): `--path` flag → `NETE_PATH` env → `~/.nete`. DB file: `{store_path}/store.db`.

**Note scoping**: `flagNote=0` = global, non-zero = specific note container.

**Links**: Single INSERT into `links` table. Both directions queried via `WHERE source_id=? OR target_id=?`. BFS traversal capped at depth 5, 1000 results.

**Memo layers**: `concrete` (facts) vs `abstract` (insights). `nete reflect` analyzes concrete memos and suggests abstract ones.

**FTS5 search**: `memos_fts` virtual table synced via triggers. BM25 ranking (title=10, content=1, tags=5, summary=3). Tag filtering via `json_each()`.

**Output contract**: stdout = pure data, stderr = status/errors.

**Multi-agent skills**: `nete init` generates instruction files for AI tools. Content is shared via `neteInstructionContent` in `cmd/skill_cmd.go`.

**Web GUI**: `nete serve` embeds `cmd/web/` (HTML+CSS+JS) via `go:embed`. API at `/api/`. Default bind: `127.0.0.1:8080`.

### Storage

```
{store_path}/
└── store.db     # SQLite: notes, memos, memos_fts, links, trash, config
```

### Module Path

`github.com/sheeppattern/nete` — all internal imports use this prefix.
