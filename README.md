# zk — AI 에이전트용 제텔카스텐 메모리 CLI / Zettelkasten Memory CLI for AI Agents

AI 에이전트가 지식을 **원자적 노트**로 저장하고, **관계 타입과 가중치가 있는 양방향 연결**로 구조화하며, **프로젝트 단위로 격리**하여 관리하는 CLI 도구.

A CLI tool that lets AI agents store knowledge as **atomic notes**, structure them with **typed and weighted bidirectional links**, and manage them in **project-scoped isolation**.

## 왜 필요한가 / Why

기존 AI 에이전트의 메모리는 단편적이다. "A를 안다"는 기억할 수 있지만, "A는 B를 뒷받침하고, B는 C와 모순된다"는 표현할 수 없다.

Conventional AI agent memory is flat. It can remember "A exists" but cannot express "A supports B, and B contradicts C."

zk는 제텔카스텐 원칙을 AI 에이전트에 적용하여 **기억 사이의 관계를 구조화**한다.

zk applies Zettelkasten principles to AI agents, enabling **structured reasoning across memories**.

```
단순 메모리 / Flat memory:   A, B, C  (고립된 사실 / isolated facts)
zk 메모리 / zk memory:      A --supports(0.9)--> B --contradicts(0.8)--> C  (구조화된 사고 / structured thought)
```

## 설치 / Installation

Go가 설치된 환경에서 / With Go installed:

```bash
go install github.com/sheeppattern/zk@latest
```

또는 바이너리 직접 빌드 / Or build from source:

```bash
git clone https://github.com/sheeppattern/zk.git
cd zk
go build -o zk .
```

## 빠른 시작 / Quick Start

```bash
# 저장소 초기화 (skill 파일도 자동 생성)
# Initialize store (also auto-generates skill files at ~/.claude/skills/zk/)
zk init

# 프로젝트 생성 / Create project
zk project create "auth-migration" --description "인증 시스템 마이그레이션"

# 노트 생성 / Create notes
zk note create --title "JWT 토큰 구조" \
  --content "Access Token과 Refresh Token 분리 저장. Redis 권장." \
  --tags "jwt,auth,redis" \
  --project P-XXXXXX

zk note create --title "세션 기반 인증의 한계" \
  --content "서버 확장 시 세션 공유 문제. Sticky session은 SPOF 위험." \
  --tags "session,auth" \
  --project P-XXXXXX

# 관계 연결 (중복 자동 방지) / Link with relation type (duplicates auto-prevented)
zk link add N-AAAAAA N-BBBBBB --type contradicts --weight 0.8 --project P-XXXXXX

# 검색 / Search
zk search "인증" --project P-XXXXXX --format md

# 연결 조회 (역참조 + 깊이 탐색) / List links (backlinks + depth traversal)
zk link list N-AAAAAA --project P-XXXXXX --depth 2
```

## 명령어 레퍼런스 / Command Reference

### 초기화 및 설정 / Init & Config

```bash
zk init                         # 저장소 초기화 + skill 파일 생성 / Initialize store + generate skill files
zk init --path /custom/path     # 커스텀 경로 / Custom path

zk config show                  # 현재 설정 조회 / Show current config
zk config set <key> <value>     # 설정 변경 / Change setting
```

설정 가능한 키 / Available config keys: `store_path`, `default_project`, `default_format`

`default_project`를 설정하면 `--project` 플래그 없이도 해당 프로젝트가 자동 적용됩니다.

When `default_project` is set, it is automatically applied when `--project` is not specified.

### 프로젝트 관리 / Project Management

```bash
zk project create <name> --description "설명"
zk project list
zk project get <id>       # 노트 수, 링크 수, 최근 활동 통계 포함 / Includes note count, link count, last activity
zk project delete <id>
```

### 노트 CRUD / Note CRUD

```bash
zk note create --title "제목" --content "내용" --tags "tag1,tag2" --project <id>
zk note get <noteID> --project <id>
zk note list --project <id>
zk note update <noteID> --title "새 제목" --content "새 내용" --project <id>
zk note delete <noteID> --project <id>           # 백링크 있으면 거부 / Refused if backlinks exist
zk note delete <noteID> --force --project <id>   # 강제 삭제 (trash로 이동) / Force delete (moved to trash)
```

삭제된 노트는 영구 삭제되지 않고 `trash/` 디렉토리로 이동합니다.

Deleted notes are not permanently removed; they are moved to the `trash/` directory.

### 링크 (관계 타입 + 가중치) / Links (Relation Type + Weight)

```bash
# 같은 프로젝트 내 연결 / Link within same project
zk link add <sourceID> <targetID> --type supports --weight 0.8 --project <id>

# 크로스 프로젝트 연결 / Cross-project link
zk link add <sourceID> <targetID> --type extends --weight 0.9 \
  --project <sourceProject> --target-project <targetProject>

zk link remove <sourceID> <targetID> --project <id>

# 연결 조회 / List links
zk link list <noteID> --project <id>                        # 직접 연결 / Direct links
zk link list <noteID> --type supports --project <id>        # 관계 타입 필터 / Filter by type
zk link list <noteID> --sort-weight --project <id>          # 가중치 순 정렬 / Sort by weight
zk link list <noteID> --depth 3 --project <id>              # BFS 깊이 탐색 / BFS traversal up to depth 3
```

중복 링크는 자동으로 방지됩니다. / Duplicate links are automatically prevented.

#### 관계 타입 / Relation Types

| 타입 / Type | 의미 / Meaning | 사용 예시 / Example |
|------|------|----------|
| `related` | 일반적 관련 (기본값) / General relation (default) | 같은 주제의 다른 관점 / Different angles of same topic |
| `supports` | 뒷받침/근거 / Evidence, backing | 증거가 주장을 지지 / Evidence supports a claim |
| `contradicts` | 반박/모순 / Contradiction | 상충하는 의견이나 접근 / Conflicting opinions |
| `extends` | 확장/발전 / Extension | 아이디어를 더 발전시킴 / Develops an idea further |
| `causes` | 원인/결과 / Causation | 인과 관계 / Cause-effect relationship |
| `example-of` | 사례/예시 / Instance | 개념의 구체적 사례 / Concrete example of a concept |

#### 가중치 / Weight

| 범위 / Range | 의미 / Meaning |
|------|------|
| 0.8~1.0 | 매우 강한 관계 (핵심 연결) / Very strong (core connection) |
| 0.5~0.7 | 보통 관계 (참고 수준) / Moderate (reference level) |
| 0.1~0.4 | 약한 관계 (간접 연결) / Weak (indirect connection) |

### 검색 / Search

```bash
zk search <query> --project <id>
zk search "Redis" --tags "cache,performance" --sort relevance
zk search "인증" --relation contradicts --min-weight 0.7
zk search "발견" --created-after 2026-01-01 --created-before 2026-12-31
```

| 옵션 / Option | 설명 / Description |
|------|------|
| `--tags` | 태그 필터 (AND 로직) / Tag filter (AND logic) |
| `--relation` | 특정 관계 타입을 가진 노트만 / Notes with specific relation type |
| `--min-weight` | 최소 가중치 이상인 링크를 가진 노트만 / Notes with link weight >= value |
| `--status` | 상태 필터 (active/archived) / Status filter |
| `--sort` | 정렬 기준 (relevance/created/updated) / Sort order |
| `--created-after` | 이 날짜 이후 생성된 노트 (YYYY-MM-DD) / Notes created on or after date |
| `--created-before` | 이 날짜 이전 생성된 노트 (YYYY-MM-DD) / Notes created on or before date |

### 태그 관리 / Tag Management

```bash
zk tag add <noteID> <tag1> [tag2...] --project <id>
zk tag remove <noteID> <tag1> [tag2...]
zk tag replace <oldTag> <newTag> --project <id>    # 프로젝트 전체 일괄 교체 / Bulk replace across project
zk tag list --project <id>                          # 모든 태그 조회 / List all unique tags
zk tag batch-add <tag> <noteID1> [noteID2...]       # 여러 노트에 일괄 추가 / Add tag to multiple notes
```

### 진단 / Diagnostics

```bash
zk diagnose --project <id>
zk diagnose --project <id> --format md
```

검사 항목 / Checks:
- 끊어진 링크 / Broken links (error)
- 파싱 실패한 노트 파일 / Corrupted note files (error)
- 고아 노트 / Orphan notes (warning)
- 잘못된 관계 타입 / Invalid relation types (warning)
- 범위 초과 가중치 / Out-of-range weights (error)

### 내보내기 / 가져오기 / Export & Import

```bash
# 내보내기 / Export
zk export --project <id> --format yaml --output backup.yaml
zk export --project <id> --notes N-AAAAAA,N-BBBBBB   # 선택 노트만 / Selected notes only

# 가져오기 / Import
zk import --file backup.yaml --project <id> --conflict skip
```

충돌 처리 옵션 / Conflict resolution: `skip` (건너뛰기 / skip), `overwrite` (덮어쓰기 / overwrite), `new-id` (새 ID 부여 / assign new ID)

### 스키마 자가 조회 / Schema Introspection

```bash
zk schema                  # 전체 리소스 목록 / List all resources
zk schema note             # 노트 필드 상세 / Note field details
zk schema link             # 링크 필드 상세 / Link field details
zk schema relation-types   # 관계 타입 목록 / Relation type list
```

AI 에이전트가 런타임에 데이터 구조를 자가 학습할 수 있다. / Enables AI agents to discover data structures at runtime.

### AI 에이전트 스킬 생성 / AI Agent Skill Generation

```bash
zk skill generate                          # ~/.claude/skills/zk/ 에 생성 / Generate to default path
zk skill generate --output /custom/path    # 커스텀 경로 / Custom path
```

`zk init` 실행 시에도 `~/.claude/skills/zk/`에 자동 생성됩니다.

Also auto-generated during `zk init` at `~/.claude/skills/zk/`.

`SKILL.md`와 `references/domain-guide.md`를 생성하여 AI 에이전트가 `/zk` 슬래시 커맨드로 CLI 사용법을 네이티브하게 로드할 수 있게 한다.

Generates `SKILL.md` and `references/domain-guide.md` so AI agents can natively load CLI usage via `/zk` slash command.

## 글로벌 옵션 / Global Options

| 옵션 / Option | 설명 / Description | 기본값 / Default |
|------|------|--------|
| `--format` | 출력 형식 / Output format (json/yaml/md) | json |
| `--project` | 프로젝트 범위 지정 / Project scope | (global) |
| `--verbose` | 디버그 출력 / Debug output to stderr | false |
| `--quiet` | stderr 상태 메시지 억제 / Suppress stderr status messages | false |

## 파이프라인 안전 출력 / Pipeline-Safe Output

- **stdout**: 순수 데이터만 (JSON/YAML/Markdown) / Pure data only
- **stderr**: 상태 메시지, 에러, 디버그 정보 / Status, errors, debug info
- `--quiet`로 stderr 상태 메시지를 억제할 수 있다 / Use `--quiet` to suppress stderr status messages

```bash
# 에이전트가 결과를 파싱하는 예시 / Agent parsing example
NOTES=$(zk search "Redis" --project P-XXXXXX --format json --quiet 2>/dev/null)
NOTE_ID=$(zk note create --title "새 발견" --content "..." --quiet 2>/dev/null | jq -r '.id')
```

## 저장소 구조 / Storage Layout

```
~/.zk-memory/
├── config.yaml
├── projects/
│   └── {project-id}/
│       ├── project.yaml
│       └── notes/
│           └── {note-id}.md      # YAML frontmatter + Markdown 본문 / body
├── global/
│   └── notes/                     # 프로젝트에 속하지 않는 범용 노트 / Project-less notes
└── trash/                          # 삭제된 노트 보관 / Soft-deleted notes
```

### 노트 파일 형식 / Note File Format

```markdown
---
id: N-72F576
title: JWT 토큰 구조
tags: [jwt, auth, redis]
links:
  - target_id: N-CE12CD
    relation_type: contradicts
    weight: 0.8
metadata:
  created_at: 2026-03-20T13:39:56+09:00
  updated_at: 2026-03-20T13:40:13+09:00
  source: ""
  status: active
project_id: P-20FFD1
---
Access Token과 Refresh Token의 분리 저장 방식 검토. Redis에 Refresh Token 저장 권장.
```

사람이 직접 읽고 편집할 수 있는 형식이다. / Human-readable and hand-editable format.

## 기술 스택 / Tech Stack

- **언어 / Language**: Go 1.26
- **CLI 프레임워크 / CLI Framework**: [cobra](https://github.com/spf13/cobra)
- **설정 관리 / Config**: [viper](https://github.com/spf13/viper)
- **YAML**: gopkg.in/yaml.v3
- **ID 생성 / ID Generation**: google/uuid

싱글 바이너리, 런타임 의존성 없음. / Single binary, zero runtime dependencies.

## 라이선스 / License

MIT

---

## 부록: 이 프로젝트는 Manyfast로 만들어졌습니다 / Appendix: Built with Manyfast

이 프로젝트의 기획 문서(PRD, 요구사항, 기능, 스펙)는 [Manyfast](https://manyfast.io)로 작성 및 관리되었습니다.

The planning documents (PRD, requirements, features, specs) for this project were created and managed with [Manyfast](https://manyfast.io).

### 기획 → 개발 과정 / Planning to Development

1. **Manyfast에서 PRD 작성 / PRD in Manyfast**: 제품 목표, 사용자 문제, 솔루션, 차별점, KPI, 리스크 정의 / Product goals, user problems, solutions, differentiation, KPIs, risks
2. **요구사항 12개 정의 / 12 Requirements defined**: PRD 기반으로 요구사항 → 기능 → 스펙 계층 구조 작성 / Hierarchical requirement → feature → spec structure
3. **AI 에이전트와 협업 개발 / AI-assisted development**: Manyfast CLI(`manyfast project get`)로 기획 문서를 읽고, 요구사항별 진행도를 `manyfast requirement write --mode update`로 실시간 추적 / Real-time progress tracking via Manyfast CLI
4. **MVP 완료 / MVP complete**: 기획 문서 작성부터 전체 요구사항 구현까지 약 **30~40분** 소요 / ~30-40 minutes from planning to full implementation

### 최종 산출물 / Final Deliverables

| 항목 / Item | 수량 / Count |
|------|------|
| 요구사항 / Requirements | 12 (전체 done / all done) |
| 기능 / Features | 24 |
| 스펙 / Specs | 48+ |
| CLI 명령어 / CLI Commands | 30+ |
| Go 소스 파일 / Go Source Files | 15 |

Manyfast 프로젝트 ID / Project ID: `5fc2a8ca-c59b-4fb3-a0c7-c5744137028b`
