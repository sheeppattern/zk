# nete — AI 에이전트용 제텔카스텐 메모리 CLI

AI 에이전트가 지식을 **원자적 메모**로 저장하고, **관계 타입과 가중치가 있는 링크**로 구조화하며, **노트 단위로 격리**하여 관리하는 CLI 도구. SQLite + FTS5 풀텍스트 검색과 웹 GUI를 내장한다.

> **nete** is a CLI tool that lets AI agents store knowledge as **atomic memos**, structure them with **typed and weighted links**, and manage them in **note-scoped isolation**. Built on SQLite with FTS5 full-text search and a built-in web GUI.

## 설계철학 — 구조화된 혼돈 (Structured Chaos)

AI 모델은 뛰어난 추론 능력과 연결성을 보여주며, 때로는 사람보다 기발한 도약을 만들어냅니다. 하지만 세션이 종료되면 그 모든 사고 과정이 휘발된다는 한계가 있습니다. 훌륭한 추론을 마친 직후의 대화에서 다시 처음부터 시작해야 하는 현상은, AI가 '생각하지 못해서'가 아니라 '기억하지 못해서' 발생합니다.

nete는 이러한 문제를 해결하기 위해 고안된 AI 에이전트의 외부 기억장치입니다. 단순한 메모 저장소를 넘어, 제텔카스텐(Zettelkasten) 방법론을 차용했습니다. 사실들을 원자적(atomic) 단위의 메모로 분할하고, 각 메모 사이에 타입과 가중치가 부여된 링크를 연결하여 지식 그래프를 구축합니다. 하나의 생각을 하나의 메모로 쪼개고 그 관계를 명시적으로 기록하는 것, 이것이 AI의 사고 과정을 가장 자연스럽게 보존하는 구조이기 때문입니다. 이를 통해 에이전트가 어떤 맥락에서 사고했고 어떻게 결론에 도달했는지, 그 사고의 과정 자체가 구조화되어 남습니다.

### 할루시네이션의 재해석: 버그가 아닌 창발성의 씨앗

일반적으로 AI의 할루시네이션(사실에 근거하지 않은 추론이나 존재하지 않는 연결)은 피해야 할 오류로 여겨집니다. 하지만 nete는 이러한 현상이 적절한 구조 내에 배치될 때, 오히려 새로운 창발성의 씨앗이 될 수 있다고 봅니다.

이를 다루기 위해 nete는 정보를 두 개의 레이어로 분리합니다.

```
Concrete Layer:  "MAU 500" ──supports──▶ "Redis 캐싱 적용"
                      │                        │
                  abstracts               contradicts
                      ▼                        ▼
Abstract Layer:  "성장 vs 리텐션 — 어느 쪽에 투자할 것인가?"
```

- **Concrete Layer** (구체적 레이어): 관찰, 데이터, 검증된 정보 등 명확한 '사실'을 기록합니다.
- **Abstract Layer** (추상적 레이어): 인사이트, 질문, 긴장감, 그리고 할루시네이션까지 포함된 '사고의 흔적'을 기록합니다.

에이전트가 사실을 기반으로 도약을 시도할 때, 그 방향의 옳고 그름을 떠나 도약 자체를 기록으로 남깁니다. 당장은 근거 없는 비약처럼 보이더라도 추후 훌륭한 아이디어로 발전할 수 있기 때문입니다. 기록해 두지 않으면 검증할 기회조차 잃게 됩니다.

이러한 도약은 무작위로 추출된 메모들이 예상치 못한 조합을 이룰 때 진가를 발휘합니다. 할루시네이션이 기존의 구체적(Concrete) 사실들과 충돌하며 새로운 질문을 파생시키고, 그 질문이 곧 새로운 탐색의 이정표가 됩니다.

### 세렌디피티: 예상치 못한 연결의 발견

메모들을 무작위로 추출하여 "이 개념들 사이에 어떤 연관성이 있지 않을까?" 하고 탐색해 보는 과정, 이것이 바로 nete가 추구하는 **구조화된 혼돈**입니다. 사람은 선입견으로 인해 시도하지 않을 조합도 AI 에이전트는 편견 없이 시도합니다. 대부분은 의미 없는 연결일 수 있지만, 그중 일부가 누구도 생각지 못한 통찰로 이어진다면 그것만으로도 충분한 가치를 지닙니다.

또한, nete는 틀린 연결이나 검증에 실패한 추론도 폐기하지 않습니다. 이를 `contradicts`나 `invalidates`와 같은 링크로 남겨, "이 방향은 유효하지 않았다"는 사실 자체를 또 다른 탐색의 방향을 지시하는 이정표로 활용합니다. 헛다리를 짚은 기록조차도 가치가 있습니다.

노이즈가 누적될 우려에 대해서는 링크의 타입과 가중치가 신호와 소음을 명확히 구분해 줍니다. 검증된 `supports` (가중치 0.9)와 실패한 `contradicts` (가중치 0.3)는 같은 공간에 존재하지만, 시스템 내에서 가지는 무게감은 확연히 다릅니다.

### 설계 원칙: 단단한 그릇

혼돈이 구조 안에서 유의미하게 작동하기 위해서는, 이를 담아내는 그릇 자체가 견고해야 합니다.

- **파이프라인 호환성**: 표준 출력(stdout)은 순수 데이터를, 표준 에러(stderr)는 상태를 반환하도록 설계하여 어떠한 CLI 도구와도 매끄럽게 조합될 수 있습니다.
- **에이전트 불가지론 (Agent Agnostic)**: stdin/stdout 기반의 CLI 구조를 채택하여 호출 방식에 제약을 받지 않습니다. Claude, GPT, Gemini 등 상용 모델이나 로컬 LLM에 종속되지 않고, 어떤 에이전트의 기억장치로든 동등하게 작동합니다.
- **단일 바이너리, 단일 파일**: 서버 구축, 도커 컨테이너, 클라우드 환경 등이 전혀 필요하지 않습니다. 오직 `nete` 실행 파일 하나와 `store.db` 데이터베이스 파일 하나만으로 완벽하게 동작합니다.

### nete가 지양하는 방향

nete는 사람을 위한 범용 노트 애플리케이션(예: Obsidian, Notion)을 대체할 목적이 아닙니다. 또한 일반적인 벡터 데이터베이스나 RAG(검색 증강 생성) 파이프라인과도 결을 달리합니다. nete는 단순한 텍스트 유사도 검색이 아닌, 노드 간의 **의미론적 관계**를 다루는 데 집중합니다. 불필요하게 복잡해지는 것을 철저히 경계하며, 본연의 철학을 흐릴 바에는 차라리 기능을 덜어냅니다.

## 설치

```bash
go install github.com/sheeppattern/nete@latest
```

또는 [GitHub Releases](https://github.com/sheeppattern/nete/releases)에서 바이너리를 다운로드.

## 빠른 시작

```bash
# 저장소 초기화 (SQLite DB + 에이전트 skill 파일 생성)
nete init

# 노트 생성 (메모를 그룹화하는 컨테이너)
nete note create "auth-migration" --description "인증 시스템 마이그레이션"

# Concrete 메모 생성 (사실 기록)
nete memo create --title "JWT 토큰 구조" \
  --content "Access Token과 Refresh Token 분리 저장." \
  --tags "jwt,auth" --layer concrete --note 1

nete memo create --title "세션 기반 인증의 한계" \
  --content "서버 확장 시 세션 공유 문제." \
  --tags "session,auth" --note 1

# 관계 연결
nete link add 1 2 --type contradicts --weight 0.8

# Abstract 메모 생성 (인사이트)
nete memo create --title "세션 vs JWT — 확장성과 복잡성의 트레이드오프" \
  --content "..." --layer abstract --note 1

# FTS5 검색
nete search "JWT"

# 인사이트 자동 제안
nete reflect --note 1

# 웹 GUI
nete serve
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
nete init                              # 저장소 초기화 (SQLite)
nete config show                       # 현재 설정 조회
nete config set default_note 1         # 기본 노트 범위
nete config set default_format yaml    # 기본 출력 형식
nete config set default_author claude  # 기본 저자
```

### 노트 관리 (컨테이너)

```bash
nete note create <name> --description "설명"
nete note list
nete note get <id>       # 메모 수, 링크 수 포함
nete note delete <id>
```

### 메모 CRUD

```bash
nete memo create --title "제목" --content "내용" --tags "t1,t2" --layer concrete --note <id>
nete memo create --title "인사이트" --content "..." --layer abstract --summary "요약" --note <id>
nete memo get <memoID>
nete memo list --note <id>
nete memo list --layer abstract --note <id>
nete memo update <memoID> --title "새 제목"
nete memo delete <memoID>
nete memo move <memoID> <targetNoteID>
nete memo random                        # 랜덤 메모 뽑기
nete memo random --layer abstract
```

### 빠른 메모

```bash
nete quickmemo "빠른 생각 기록"
nete quickmemo "관찰 사항" --note <id> --author claude
```

### 링크

```bash
nete link add <src> <tgt> --type supports --weight 0.8
nete link remove <src> <tgt> --type supports
nete link list <memoID>
nete link list <memoID> --depth 3       # BFS 탐색 (최대 depth 5)
```

### 검색 (FTS5 풀텍스트)

```bash
nete search <query>
nete search "Redis" --tags "cache"
nete search "인증" --layer abstract --note <id>
nete search "data" --created-after 2026-01-01 --sort relevance
```

BM25 랭킹: 제목(10배) > 태그(5배) > 요약(3배) > 본문(1배)

### 태그

```bash
nete tag add <memoID> <tag1> [tag2...]
nete tag remove <memoID> <tag1> [tag2...]
nete tag replace <oldTag> <newTag> --note <id>
nete tag list --note <id>
```

### 진단

```bash
nete diagnose
nete diagnose --format md
```

고아 메모, 잘못된 관계 타입, 범위 초과 가중치를 검사.

### 내보내기 / 가져오기

```bash
nete export --note <id> --format yaml --output backup.yaml
nete import --file backup.yaml --note <id>
```

### 인사이트 엔진

```bash
nete reflect --note <id>              # 인사이트 후보 출력
nete reflect --note <id> --apply      # 자동으로 메모 생성
nete reflect --note <id> --suggest-links  # 유사 메모 링크 제안
```

### 그래프 및 탐색

```bash
nete graph --note <id>                # Mermaid 그래프
nete explore <memoID> --depth 2       # 연결 탐색
```

### 웹 GUI

```bash
nete serve                            # http://127.0.0.1:8080
nete serve --addr :3000               # 커스텀 포트
```

상하 분할 레이아웃: 에디터 (Incoming | 본문 | Outgoing) + 탐색기 + 미니맵

### 스키마 조회

```bash
nete schema              # 전체 리소스 목록
nete schema memo         # 메모 필드 상세
nete schema link         # 링크 필드 상세
```

### 마이그레이션 (v0.3 → v0.4)

```bash
nete migrate ~/.nete              # 기존 .md 파일 → SQLite 변환
nete migrate ~/.nete --dry-run    # 미리보기
```

### 세렌디피티 — 교차 수분 발견

무작위 메모에서 숨겨진 연결고리를 발견하는 워크플로우.

```bash
# 1. 무작위 메모 2~5개 뽑기
nete memo random --format json   # 반복 실행

# 2. 각 메모 내용 확인
nete memo get <id> --format md

# 3. 에이전트가 연결 분석 → 서브에이전트로 논리 검증 → 링크 생성
nete link add <id1> <id2> --type related --weight 0.6
nete memo create --title "Serendipity: X connects to Y" \
  --content "..." --layer abstract --tags "serendipity"
```

- 모든 노트에서 무작위로 뽑음 (특정 노트에 한정하지 않음)
- 숨겨진 패턴, 유사성, 모순, 인과관계를 탐색
- 서브에이전트(가용 시)에게 논리적 결함 검증을 위임
- 검증된 연결만 링크로 기록, `serendipity` 태그로 추적

### 버전 관리

```bash
nete version
nete update                  # 최신 버전으로 업데이트
nete update --check          # 업데이트 확인만
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
MEMOS=$(nete search "Redis" --note 1 --format json --quiet 2>/dev/null)
```

## 저장소 구조

```
~/.nete/
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

## 부록: 이 프로젝트는 Manyfast로 만들어졌습니다

이 프로젝트의 기획 문서(PRD, 요구사항, 기능, 스펙)는 [Manyfast](https://manyfast.io?utm_source=github&utm_medium=readme&utm_campaign=nete)로 작성 및 관리되었습니다.

> Planning documents (PRD, requirements, features, specs) were created and managed with [Manyfast](https://manyfast.io?utm_source=github&utm_medium=readme&utm_campaign=nete).

### 기획에서 개발까지

1. **Manyfast에서 PRD 작성**: 제품 목표, 사용자 문제, 솔루션, 차별점, KPI, 리스크 정의
2. **요구사항 12개 정의**: PRD 기반 요구사항 → 기능 → 스펙 계층 구조
3. **AI 에이전트와 협업 개발**: Manyfast CLI로 기획 문서를 읽고, 진행도를 실시간 추적
4. **MVP 완료**: 기획 문서 작성부터 전체 구현까지 약 **30~40분** 소요

> 1. PRD in Manyfast: goals, problems, solutions, KPIs, risks
> 2. 12 requirements defined: hierarchical requirement → feature → spec structure
> 3. AI-assisted development with real-time progress tracking via Manyfast CLI
> 4. MVP complete: ~30-40 minutes from planning to full implementation

### 최종 산출물

| 항목 | 수량 |
|------|------|
| 요구사항 (Requirements) | 12 (전체 done) |
| 기능 (Features) | 24 |
| 스펙 (Specs) | 58 |
| CLI 명령어 | 35+ |
| Go 소스 파일 | 17 |
| 테스트 | 46 |

> Manyfast Project ID: `5fc2a8ca-c59b-4fb3-a0c7-c5744137028b`

## 라이선스

MIT
