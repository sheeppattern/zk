# zk — AI 에이전트용 제텔카스텐 메모리 CLI

AI 에이전트가 지식을 **원자적 노트**로 저장하고, **관계 타입과 가중치가 있는 양방향 연결**로 구조화하며, **프로젝트 단위로 격리**하여 관리하는 CLI 도구.

## 왜 필요한가

기존 AI 에이전트의 메모리는 단편적이다. "A를 안다"는 기억할 수 있지만, "A는 B를 뒷받침하고, B는 C와 모순된다"는 표현할 수 없다.

zk는 제텔카스텐 원칙을 AI 에이전트에 적용하여 **기억 사이의 관계를 구조화**한다.

```
단순 메모리:  A, B, C  (고립된 사실)
zk 메모리:   A --supports(0.9)--> B --contradicts(0.8)--> C  (구조화된 사고)
```

## 설치

Go가 설치된 환경에서:

```bash
go install github.com/sheeppattern/zk@latest
```

또는 바이너리 직접 빌드:

```bash
git clone https://github.com/sheeppattern/zk.git
cd zk
go build -o zk .
```

## 빠른 시작

```bash
# 저장소 초기화
zk init

# 프로젝트 생성
zk project create "auth-migration" --description "인증 시스템 마이그레이션"

# 노트 생성
zk note create --title "JWT 토큰 구조" \
  --content "Access Token과 Refresh Token 분리 저장. Redis 권장." \
  --tags "jwt,auth,redis" \
  --project P-XXXXXX

zk note create --title "세션 기반 인증의 한계" \
  --content "서버 확장 시 세션 공유 문제. Sticky session은 SPOF 위험." \
  --tags "session,auth" \
  --project P-XXXXXX

# 관계 연결 (JWT가 세션의 한계를 해결한다)
zk link add N-AAAAAA N-BBBBBB --type contradicts --weight 0.8 --project P-XXXXXX

# 검색
zk search "인증" --project P-XXXXXX --format md

# 연결 조회 (역참조 포함)
zk link list N-AAAAAA --project P-XXXXXX
```

## 명령어 레퍼런스

### 초기화 및 설정

```bash
zk init                         # 저장소 초기화
zk init --path /custom/path     # 커스텀 경로
```

### 프로젝트 관리

```bash
zk project create <name> --description "설명"
zk project list
zk project get <id>
zk project delete <id>
```

### 노트 CRUD

```bash
zk note create --title "제목" --content "내용" --tags "tag1,tag2" --project <id>
zk note get <noteID> --project <id>
zk note list --project <id>
zk note update <noteID> --title "새 제목" --content "새 내용" --project <id>
zk note delete <noteID> --project <id>
```

### 링크 (관계 타입 + 가중치)

```bash
# 같은 프로젝트 내 연결
zk link add <sourceID> <targetID> --type supports --weight 0.8 --project <id>

# 크로스 프로젝트 연결
zk link add <sourceID> <targetID> --type extends --weight 0.9 \
  --project <sourceProject> --target-project <targetProject>

zk link remove <sourceID> <targetID> --project <id>
zk link list <noteID> --project <id>
```

#### 관계 타입

| 타입 | 의미 | 사용 예시 |
|------|------|----------|
| `related` | 일반적 관련 (기본값) | 같은 주제의 다른 관점 |
| `supports` | 뒷받침/근거 | 증거가 주장을 지지 |
| `contradicts` | 반박/모순 | 상충하는 의견이나 접근 |
| `extends` | 확장/발전 | 아이디어를 더 발전시킴 |
| `causes` | 원인/결과 | 인과 관계 |
| `example-of` | 사례/예시 | 개념의 구체적 사례 |

#### 가중치

| 범위 | 의미 |
|------|------|
| 0.8~1.0 | 매우 강한 관계 (핵심 연결) |
| 0.5~0.7 | 보통 관계 (참고 수준) |
| 0.1~0.4 | 약한 관계 (간접 연결) |

### 검색

```bash
zk search <query> --project <id>
zk search "Redis" --tags "cache,performance" --sort relevance
zk search "인증" --relation contradicts --min-weight 0.7
```

| 옵션 | 설명 |
|------|------|
| `--tags` | 태그 필터 (AND 로직) |
| `--relation` | 특정 관계 타입을 가진 노트만 |
| `--min-weight` | 최소 가중치 이상인 링크를 가진 노트만 |
| `--status` | 상태 필터 (active/archived) |
| `--sort` | 정렬 기준 (relevance/created/updated) |

### 태그 관리

```bash
zk tag add <noteID> <tag1> [tag2...] --project <id>
zk tag remove <noteID> <tag1> [tag2...]
zk tag replace <oldTag> <newTag> --project <id>    # 프로젝트 전체 일괄 교체
zk tag list --project <id>                          # 모든 태그 조회
zk tag batch-add <tag> <noteID1> [noteID2...]       # 여러 노트에 일괄 추가
```

### 진단

```bash
zk diagnose --project <id>
zk diagnose --project <id> --format md
```

끊어진 링크, 고아 노트, 잘못된 관계 타입, 범위 초과 가중치를 검사하고 오류/경고를 구분하여 리포트한다.

### 내보내기 / 가져오기

```bash
# 내보내기
zk export --project <id> --format yaml --output backup.yaml
zk export --project <id> --notes N-AAAAAA,N-BBBBBB   # 선택 노트만

# 가져오기
zk import --file backup.yaml --project <id> --conflict skip
```

충돌 처리 옵션: `skip` (건너뛰기), `overwrite` (덮어쓰기), `new-id` (새 ID 부여)

### 스키마 자가 조회

```bash
zk schema                  # 전체 리소스 목록
zk schema note             # 노트 필드 상세
zk schema link             # 링크 필드 상세
zk schema relation-types   # 관계 타입 목록
```

AI 에이전트가 런타임에 데이터 구조를 학습할 수 있다.

### AI 에이전트 스킬 생성

```bash
zk skill generate                          # ~/.claude/skills/zk/ 에 생성
zk skill generate --output /custom/path    # 커스텀 경로
```

`SKILL.md`와 `references/domain-guide.md`를 자동 생성하여 AI 에이전트가 `/zk` 슬래시 커맨드로 CLI 사용법을 네이티브하게 로드할 수 있게 한다.

## 글로벌 옵션

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--format` | 출력 형식 (json/yaml/md) | json |
| `--project` | 프로젝트 범위 지정 | (global) |
| `--verbose` | 디버그 출력 | false |

## 파이프라인 안전 출력

- **stdout**: 순수 데이터만 (JSON/YAML/Markdown)
- **stderr**: 상태 메시지, 에러, 디버그 정보

```bash
# 에이전트가 결과를 파싱하는 예시
NOTES=$(zk search "Redis" --project P-XXXXXX --format json 2>/dev/null)
NOTE_ID=$(zk note create --title "새 발견" --content "..." 2>/dev/null | jq -r '.id')
```

## 저장소 구조

```
~/.zk-memory/
├── config.yaml
├── projects/
│   └── {project-id}/
│       ├── project.yaml
│       └── notes/
│           └── {note-id}.md      # YAML frontmatter + Markdown 본문
└── global/
    └── notes/                     # 프로젝트에 속하지 않는 범용 노트
```

### 노트 파일 형식

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

사람이 직접 읽고 편집할 수 있는 형식이다.

## 기술 스택

- **언어**: Go 1.26
- **CLI 프레임워크**: [cobra](https://github.com/spf13/cobra)
- **설정 관리**: [viper](https://github.com/spf13/viper)
- **YAML**: gopkg.in/yaml.v3
- **ID 생성**: google/uuid

싱글 바이너리, 런타임 의존성 없음.

## 라이선스

MIT

---

## 부록: 이 프로젝트는 Manyfast로 만들어졌습니다

이 프로젝트의 기획 문서(PRD, 요구사항, 기능, 스펙)는 [Manyfast](https://manyfast.io)로 작성 및 관리되었습니다.

### 기획 → 개발 과정

1. **Manyfast에서 PRD 작성**: 제품 목표, 사용자 문제, 솔루션, 차별점, KPI, 리스크 정의
2. **요구사항 12개 정의**: PRD 기반으로 요구사항 → 기능 → 스펙 계층 구조 작성
3. **AI 에이전트와 협업 개발**: Manyfast CLI(`manyfast project get`)로 기획 문서를 읽고, 요구사항별 진행도를 `manyfast requirement write --mode update`로 실시간 추적
4. **MVP 완료**: 기획 문서 작성부터 전체 요구사항 구현까지 약 **30~40분** 소요

### 최종 산출물

| 항목 | 수량 |
|------|------|
| 요구사항 (Requirements) | 12개 (전체 done) |
| 기능 (Features) | 24개 |
| 스펙 (Specs) | 48개+ |
| CLI 명령어 | 25개+ |
| Go 소스 파일 | 14개 |

Manyfast 프로젝트 ID: `5fc2a8ca-c59b-4fb3-a0c7-c5744137028b`
