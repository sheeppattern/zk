# zk — AI 에이전트용 제텔카스텐 메모리 CLI

AI 에이전트가 지식을 **원자적 메모**로 저장하고, **관계 타입과 가중치가 있는 링크**로 구조화하며, **노트 단위로 격리**하여 관리하는 CLI 도구. SQLite + FTS5 풀텍스트 검색과 웹 GUI를 내장한다.

> **zk** is a CLI tool that lets AI agents store knowledge as **atomic memos**, structure them with **typed and weighted links**, and manage them in **note-scoped isolation**. Built on SQLite with FTS5 full-text search and a built-in web GUI.

## 왜 필요한가

기존 AI 에이전트의 메모리는 단편적이다. zk는 제텔카스텐 원칙을 AI 에이전트에 적용하여 **기억 사이의 관계를 구조화**하고, **사실(concrete)에서 인사이트(abstract)를 체계적으로 도출**한다.

```
Concrete Layer:  "MAU 500" ──supports──▶ "Redis 캐싱 적용"
                      │                        │
                  abstracts               contradicts
                      ▼                        ▼
Abstract Layer:  "성장 vs 리텐션 — 어느 쪽에 투자할 것인가?"
```

## 설치

```bash
go install github.com/sheeppattern/zk@latest
```

또는 [GitHub Releases](https://github.com/sheeppattern/zk/releases)에서 바이너리를 다운로드.

## 빠른 시작

```bash
# 저장소 초기화 (SQLite DB + 에이전트 skill 파일 생성)
zk init

# 노트 생성 (메모를 그룹화하는 컨테이너)
zk note create "auth-migration" --description "인증 시스템 마이그레이션"

# Concrete 메모 생성 (사실 기록)
zk memo create --title "JWT 토큰 구조" \
  --content "Access Token과 Refresh Token 분리 저장." \
  --tags "jwt,auth" --layer concrete --note 1

zk memo create --title "세션 기반 인증의 한계" \
  --content "서버 확장 시 세션 공유 문제." \
  --tags "session,auth" --note 1

# 관계 연결
zk link add 1 2 --type contradicts --weight 0.8

# Abstract 메모 생성 (인사이트)
zk memo create --title "세션 vs JWT — 확장성과 복잡성의 트레이드오프" \
  --content "..." --layer abstract --note 1

# FTS5 검색
zk search "JWT"

# 인사이트 자동 제안
zk reflect --note 1

# 웹 GUI
zk serve
```

## 핵심 개념

### 노트와 메모

| 개념 | 역할 | 예시 |
|------|------|------|
| **Note** | 메모를 그룹화하는 컨테이너 (폴더) | "auth-migration", "research" |
| **Memo** | 원자적 지식 기록 (실제 내용) | "JWT 토큰 구조", "세션의 한계" |

### Concrete/Abstract 레이어

| 레이어 | 역할 | 예시 |
|--------|------|------|
| **concrete** (기본) | 사실, 관찰, 데이터 기록 | "MAU 500, 리텐션 23%" |
| **abstract** | 패턴, 긴장, 질문, 인사이트 | "성장 투자 vs 리텐션 개선" |

### 링크

링크는 **단일 저장**되고 양방향으로 조회된다 (중복 없음).

| 관계 타입 | 의미 |
|-----------|------|
| `related` | 일반적 관련 (기본값) |
| `supports` | 뒷받침/근거 |
| `contradicts` | 반박/모순 |
| `extends` | 확장/발전 |
| `causes` | 원인/결과 |
| `example-of` | 사례/예시 |
| `abstracts` | concrete → abstract 도출 |
| `grounds` | abstract의 근거가 되는 concrete |
| `replaces` | 새 메모가 기존을 대체 |
| `invalidates` | 데이터가 가설을 반증 |

가중치: 0.8–1.0 (핵심), 0.5–0.7 (참고), 0.1–0.4 (간접)

## 명령어 레퍼런스

### 초기화 및 설정

```bash
zk init                              # 저장소 초기화 (SQLite)
zk config show                       # 현재 설정 조회
zk config set default_note 1         # 기본 노트 범위
zk config set default_format yaml    # 기본 출력 형식
zk config set default_author claude  # 기본 저자
```

### 노트 관리 (컨테이너)

```bash
zk note create <name> --description "설명"
zk note list
zk note get <id>       # 메모 수, 링크 수 포함
zk note delete <id>
```

### 메모 CRUD

```bash
zk memo create --title "제목" --content "내용" --tags "t1,t2" --layer concrete --note <id>
zk memo create --title "인사이트" --content "..." --layer abstract --summary "요약" --note <id>
zk memo get <memoID>
zk memo list --note <id>
zk memo list --layer abstract --note <id>
zk memo update <memoID> --title "새 제목"
zk memo delete <memoID>
zk memo move <memoID> <targetNoteID>
zk memo random                        # 랜덤 메모 뽑기
zk memo random --layer abstract
```

### 빠른 메모

```bash
zk quickmemo "빠른 생각 기록"
zk quickmemo "관찰 사항" --note <id> --author claude
```

### 링크

```bash
zk link add <src> <tgt> --type supports --weight 0.8
zk link remove <src> <tgt> --type supports
zk link list <memoID>
zk link list <memoID> --depth 3       # BFS 탐색 (최대 depth 5)
```

### 검색 (FTS5 풀텍스트)

```bash
zk search <query>
zk search "Redis" --tags "cache"
zk search "인증" --layer abstract --note <id>
zk search "data" --created-after 2026-01-01 --sort relevance
```

BM25 랭킹: 제목(10배) > 태그(5배) > 요약(3배) > 본문(1배)

### 태그

```bash
zk tag add <memoID> <tag1> [tag2...]
zk tag remove <memoID> <tag1> [tag2...]
zk tag replace <oldTag> <newTag> --note <id>
zk tag list --note <id>
```

### 진단

```bash
zk diagnose
zk diagnose --format md
```

고아 메모, 잘못된 관계 타입, 범위 초과 가중치를 검사.

### 내보내기 / 가져오기

```bash
zk export --note <id> --format yaml --output backup.yaml
zk import --file backup.yaml --note <id>
```

### 인사이트 엔진

```bash
zk reflect --note <id>              # 인사이트 후보 출력
zk reflect --note <id> --apply      # 자동으로 메모 생성
zk reflect --note <id> --suggest-links  # 유사 메모 링크 제안
```

### 그래프 및 탐색

```bash
zk graph --note <id>                # Mermaid 그래프
zk explore <memoID> --depth 2       # 연결 탐색
```

### 웹 GUI

```bash
zk serve                            # http://127.0.0.1:8080
zk serve --addr :3000               # 커스텀 포트
```

상하 분할 레이아웃: 에디터 (Incoming | 본문 | Outgoing) + 탐색기 + 미니맵

### 스키마 조회

```bash
zk schema              # 전체 리소스 목록
zk schema memo         # 메모 필드 상세
zk schema link         # 링크 필드 상세
```

### 마이그레이션 (v0.3 → v0.4)

```bash
zk migrate ~/.zk-memory              # 기존 .md 파일 → SQLite 변환
zk migrate ~/.zk-memory --dry-run    # 미리보기
```

### 버전 관리

```bash
zk version
zk update                  # 최신 버전으로 업데이트
zk update --check          # 업데이트 확인만
```

## 글로벌 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--format` | 출력 형식 (json/yaml/md) | json |
| `--note` | 노트 범위 지정 (0=글로벌) | 0 |
| `--verbose` | 디버그 출력 | false |
| `--quiet` | stderr 상태 메시지 억제 | false |

## 파이프라인 안전 출력

- **stdout**: 순수 데이터만 (JSON/YAML/Markdown)
- **stderr**: 상태 메시지, 에러, 디버그 정보

```bash
MEMOS=$(zk search "Redis" --note 1 --format json --quiet 2>/dev/null)
```

## 저장소 구조

```
~/.zk-memory/
└── store.db     # 단일 SQLite 데이터베이스
```

테이블: `notes`, `memos`, `memos_fts` (FTS5), `links`, `trash`, `config`

## 기술 스택

- **언어**: Go 1.26
- **CLI 프레임워크**: [cobra](https://github.com/spf13/cobra)
- **데이터베이스**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **풀텍스트 검색**: SQLite FTS5 + BM25 랭킹
- **설정 관리**: [viper](https://github.com/spf13/viper)

> Single binary, zero runtime dependencies.

## 라이선스

MIT
