package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var skillOutputDir string

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage zk skill definitions",
}

var skillGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate skill files for Claude Code integration",
	RunE:  runSkillGenerate,
}

func init() {
	home, err := os.UserHomeDir()
	defaultOutput := filepath.Join("~", ".claude", "skills", "zk")
	if err == nil {
		defaultOutput = filepath.Join(home, ".claude", "skills", "zk")
	}

	skillGenerateCmd.Flags().StringVar(&skillOutputDir, "output", defaultOutput, "output directory for skill files")
	skillCmd.AddCommand(skillGenerateCmd)
	rootCmd.AddCommand(skillCmd)
}

func runSkillGenerate(cmd *cobra.Command, args []string) error {
	outDir := skillOutputDir
	if strings.HasPrefix(outDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		outDir = filepath.Join(home, outDir[1:])
	}

	// 1. Create output directory
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 2. Write SKILL.md
	skillPath := filepath.Join(outDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillMDContent), 0o644); err != nil {
		return fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	// 3. Create references/ subdirectory
	refsDir := filepath.Join(outDir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create references directory: %w", err)
	}

	// 4. Write references/domain-guide.md
	domainPath := filepath.Join(refsDir, "domain-guide.md")
	if err := os.WriteFile(domainPath, []byte(domainGuideMDContent), 0o644); err != nil {
		return fmt.Errorf("failed to write domain-guide.md: %w", err)
	}

	// 5. Print success to stderr
	fmt.Fprintln(os.Stderr, "skill files generated successfully:")
	fmt.Fprintf(os.Stderr, "  %s\n", skillPath)
	fmt.Fprintf(os.Stderr, "  %s\n", domainPath)

	return nil
}

// bt is a shorthand for triple backticks to use inside raw string constants.
const bt = "```"

var skillMDContent = `---
name: zk
description: "Zettelkasten memory CLI — AI 에이전트용 지식 노트 관리 도구. 원자적 노트 CRUD, 양방향 연결(관계 타입+가중치), 프로젝트 범위 관리, 검색/필터링, 무결성 진단을 지원합니다."
---

# Zettelkasten Memory CLI (zk)

> AI 에이전트가 지식을 구조화하고 연결하는 CLI 도구.

## 글로벌 옵션

` + bt + `bash
--format <fmt>     # 출력 형식: json (기본) | yaml | md
--project <id>     # 프로젝트 범위 지정
--verbose          # 디버그 정보 stderr 출력
` + bt + `

## 초기화

` + bt + `bash
zk init                    # 저장소 초기화
zk init --path /custom     # 커스텀 경로
` + bt + `

## 프로젝트

` + bt + `bash
zk project create <name> --description "설명"
zk project list
zk project get <id>
zk project delete <id>
` + bt + `

## 노트

` + bt + `bash
zk note create --title "제목" --content "내용" --tags "tag1,tag2" --project <id>
zk note get <noteID> --project <id>
zk note list --project <id>
zk note update <noteID> --title "새 제목" --project <id>
zk note delete <noteID> --project <id>
` + bt + `

## 링크 (관계 타입 + 가중치)

` + bt + `bash
zk link add <sourceID> <targetID> --type supports --weight 0.8 --project <id>
zk link remove <sourceID> <targetID> --project <id>
zk link list <noteID> --project <id>
` + bt + `

관계 타입: related (기본), supports, contradicts, extends, causes, example-of

## 검색

` + bt + `bash
zk search <query> --project <id>
zk search <query> --tags "tag1,tag2" --relation supports --min-weight 0.5 --sort relevance
` + bt + `

## 태그

` + bt + `bash
zk tag add <noteID> <tag1> [tag2...] --project <id>
zk tag remove <noteID> <tag1> [tag2...]
zk tag replace <oldTag> <newTag> --project <id>
zk tag list --project <id>
zk tag batch-add <tag> <noteID1> [noteID2...]
` + bt + `

## 진단

` + bt + `bash
zk diagnose --project <id>
` + bt + `

## 내보내기 / 가져오기

` + bt + `bash
zk export --project <id> --format yaml --output backup.yaml
zk import --file backup.yaml --project <id> --conflict skip
` + bt + `

## 스키마

` + bt + `bash
zk schema              # 전체 리소스 목록
zk schema note         # 노트 스키마 상세
zk schema link         # 링크 스키마 상세
zk schema relation-types
` + bt + `

## 출력 형식

stdout: 데이터만 출력 (JSON/YAML/MD). stderr: 상태/에러. 파이프라인 안전.

## 에이전트 워크플로우

### 1. 지식 축적 흐름
` + bt + `bash
zk init
zk project create "my-research" --description "연구 프로젝트"
zk note create --title "발견 1" --content "..." --tags "finding" --project P-XXXXXX
zk note create --title "발견 2" --content "..." --tags "finding" --project P-XXXXXX
zk link add N-AAAAAA N-BBBBBB --type supports --weight 0.9 --project P-XXXXXX
` + bt + `

### 2. 지식 탐색 흐름
` + bt + `bash
zk search "키워드" --project P-XXXXXX
zk link list N-AAAAAA --project P-XXXXXX
zk note get N-BBBBBB --project P-XXXXXX
` + bt + `

### 3. 유지보수 흐름
` + bt + `bash
zk diagnose --project P-XXXXXX
zk export --project P-XXXXXX --output snapshot.yaml
` + bt + `

## 데이터 구조

` + bt + `
{StorePath}/
├── config.yaml
├── projects/
│   └── {project-id}/
│       ├── project.yaml
│       └── notes/
│           └── {note-id}.md
└── global/
    └── notes/
` + bt + `

## 주의사항

- 노트 파일은 YAML frontmatter + Markdown 본문 형식
- 링크는 양방향 자동 생성 (add 시 source→target, target→source 모두 기록)
- --project 미지정 시 global 영역에 저장
- exit code: 0=성공, 1=에러
`

var domainGuideMDContent = `# Zettelkasten 도메인 가이드

> 제텔카스텐 메모리 CLI의 도메인 지식과 베스트 프랙티스.

## 핵심 원칙

### 원자적 노트 (Atomic Notes)
- 하나의 노트 = 하나의 아이디어/정보 단위
- 너무 많은 개념을 하나의 노트에 담지 않기
- 재사용 가능한 단위로 분리

### 양방향 연결 (Bidirectional Links)
- 모든 링크는 자동으로 양방향 생성
- 관계 타입으로 "왜 연결되었는지" 표현
- 가중치로 "얼마나 강한 관계인지" 표현

## 관계 타입 사용 가이드

| 타입 | 의미 | 사용 예시 |
|------|------|----------|
| related | 일반적 관련 (기본) | 같은 주제의 다른 관점 |
| supports | 뒷받침/근거 | 증거가 주장을 지지 |
| contradicts | 반박/모순 | 상충하는 의견 |
| extends | 확장/발전 | 아이디어를 더 발전시킴 |
| causes | 원인/결과 | 인과 관계 |
| example-of | 사례/예시 | 개념의 구체적 사례 |

## 가중치 가이드

| 범위 | 의미 |
|------|------|
| 0.8~1.0 | 매우 강한 관계 (핵심 연결) |
| 0.5~0.7 | 보통 관계 (참고 수준) |
| 0.1~0.4 | 약한 관계 (간접 연결) |

## 베스트 프랙티스

1. **프로젝트로 맥락 격리**: 관련 노트는 같은 프로젝트에
2. **태그로 횡단 분류**: 프로젝트를 넘나드는 주제는 태그로
3. **정기 진단**: ` + "`zk diagnose`" + `로 끊어진 링크 확인
4. **백업**: ` + "`zk export`" + `로 정기 스냅샷
5. **관계 타입 활용**: "related"만 쓰지 말고 구체적 관계 표현
`
