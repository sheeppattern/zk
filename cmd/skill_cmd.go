package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage zk skill definitions for AI coding agents",
}

var skillGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate instruction files for AI coding agents",
	Long: `Generate instruction/skill files for multiple AI coding agents.

Global files (Claude, Gemini, Codex) are always generated at ~/.
Project files (Cursor, Copilot, Windsurf) require --project-dir.`,
	Example: `  zk skill generate
  zk skill generate --project-dir .
  zk skill generate --agents claude,cursor --project-dir .
  zk skill generate --global-only`,
	RunE: runSkillGenerate,
}

func init() {
	skillGenerateCmd.Flags().String("agents", "all", "comma-separated agent targets: all, claude, gemini, codex, agent-skills, agent-skills-project, cursor, copilot, windsurf")
	skillGenerateCmd.Flags().String("project-dir", "", "project directory for project-level files (cursor, copilot, windsurf)")
	skillGenerateCmd.Flags().Bool("global-only", false, "only generate global (user-level) files")
	skillCmd.AddCommand(skillGenerateCmd)
	rootCmd.AddCommand(skillCmd)
}

// agentTarget represents a supported AI coding agent.
type agentTarget struct {
	Name    string
	Global  bool   // true = user-level (~), false = project-level
	WriteFn func(baseDir string) (string, error)
}

func allAgentTargets() []agentTarget {
	return []agentTarget{
		{Name: "claude", Global: true, WriteFn: writeClaudeSkill},
		{Name: "gemini", Global: true, WriteFn: writeGeminiInstruction},
		{Name: "codex", Global: true, WriteFn: writeCodexInstruction},
		{Name: "agent-skills", Global: true, WriteFn: writeAgentSkillsGlobal},
		{Name: "agent-skills-project", Global: false, WriteFn: writeAgentSkillsProject},
		{Name: "cursor", Global: false, WriteFn: writeCursorRule},
		{Name: "copilot", Global: false, WriteFn: writeCopilotInstruction},
		{Name: "windsurf", Global: false, WriteFn: writeWindsurfRule},
	}
}

func runSkillGenerate(cmd *cobra.Command, args []string) error {
	agentsFlag, _ := cmd.Flags().GetString("agents")
	projectDir, _ := cmd.Flags().GetString("project-dir")
	globalOnly, _ := cmd.Flags().GetBool("global-only")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	// Parse agent filter.
	selected := map[string]bool{}
	if agentsFlag == "all" {
		for _, t := range allAgentTargets() {
			selected[t.Name] = true
		}
	} else {
		for _, name := range strings.Split(agentsFlag, ",") {
			selected[strings.TrimSpace(name)] = true
		}
	}

	var generated []string

	for _, t := range allAgentTargets() {
		if !selected[t.Name] {
			continue
		}
		if t.Global {
			path, err := t.WriteFn(home)
			if err != nil {
				debugf("failed to write %s: %v", t.Name, err)
				continue
			}
			generated = append(generated, fmt.Sprintf("  %s (%s)", path, t.Name))
		} else if !globalOnly && projectDir != "" {
			path, err := t.WriteFn(projectDir)
			if err != nil {
				debugf("failed to write %s: %v", t.Name, err)
				continue
			}
			generated = append(generated, fmt.Sprintf("  %s (%s)", path, t.Name))
		}
	}

	if len(generated) > 0 {
		statusf("agent skill files generated:")
		for _, g := range generated {
			statusf("%s", g)
		}
	}

	return nil
}

// WriteGlobalAgentFiles generates only global (user-level) agent files. Called by init.
func WriteGlobalAgentFiles() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	var generated []string
	for _, t := range allAgentTargets() {
		if !t.Global {
			continue
		}
		path, err := t.WriteFn(home)
		if err != nil {
			debugf("failed to write %s: %v", t.Name, err)
			continue
		}
		generated = append(generated, fmt.Sprintf("  %s (%s)", path, t.Name))
	}

	if len(generated) > 0 {
		statusf("agent skill files generated:")
		for _, g := range generated {
			statusf("%s", g)
		}
	}
	return nil
}

// --- Agent-specific writers ---

func writeFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// Claude Code: ~/.claude/skills/zk/SKILL.md + references/domain-guide.md
func writeClaudeSkill(home string) (string, error) {
	dir := filepath.Join(home, ".claude", "skills", "zk")
	skillPath := filepath.Join(dir, "SKILL.md")

	content := claudeFrontmatter + zkInstructionContent
	if err := writeFile(skillPath, content); err != nil {
		return "", err
	}

	domainPath := filepath.Join(dir, "references", "domain-guide.md")
	if err := writeFile(domainPath, domainGuideContent); err != nil {
		return "", err
	}

	return skillPath, nil
}

// Gemini CLI: ~/.gemini/instructions/zk.md
func writeGeminiInstruction(home string) (string, error) {
	path := filepath.Join(home, ".gemini", "instructions", "zk.md")
	if err := writeFile(path, zkInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Codex CLI: ~/.codex/instructions/zk.md
func writeCodexInstruction(home string) (string, error) {
	path := filepath.Join(home, ".codex", "instructions", "zk.md")
	if err := writeFile(path, zkInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Agent Skills Standard (agentskills.io): ~/.agents/skills/zk/SKILL.md (global)
func writeAgentSkillsGlobal(home string) (string, error) {
	return writeAgentSkillsDir(filepath.Join(home, ".agents", "skills", "zk"))
}

// Agent Skills Standard (agentskills.io): {projectDir}/.agents/skills/zk/SKILL.md (project)
func writeAgentSkillsProject(projectDir string) (string, error) {
	return writeAgentSkillsDir(filepath.Join(projectDir, ".agents", "skills", "zk"))
}

func writeAgentSkillsDir(dir string) (string, error) {
	skillPath := filepath.Join(dir, "SKILL.md")

	content := claudeFrontmatter + zkInstructionContent
	if err := writeFile(skillPath, content); err != nil {
		return "", err
	}

	domainPath := filepath.Join(dir, "references", "domain-guide.md")
	if err := writeFile(domainPath, domainGuideContent); err != nil {
		return "", err
	}

	return skillPath, nil
}

// Cursor: {projectDir}/.cursor/rules/zk.mdc
func writeCursorRule(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".cursor", "rules", "zk.mdc")
	content := cursorFrontmatter + zkInstructionContent
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// GitHub Copilot: {projectDir}/.github/copilot-instructions.md
func writeCopilotInstruction(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".github", "copilot-instructions.md")
	if err := writeFile(path, zkInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Windsurf: {projectDir}/.windsurf/rules/zk.md
func writeWindsurfRule(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".windsurf", "rules", "zk.md")
	content := windsurfFrontmatter + zkInstructionContent
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// --- Frontmatter constants ---

const claudeFrontmatter = `---
name: zk
description: "Zettelkasten memory CLI — AI 에이전트용 지식 노트 관리 도구. 원자적 노트 CRUD, 양방향 연결(관계 타입+가중치), 프로젝트 범위 관리, 검색/필터링, 무결성 진단을 지원합니다."
---

`

const cursorFrontmatter = `---
description: "zk - Zettelkasten memory CLI for AI agents. Atomic note CRUD, typed+weighted bidirectional links, project scoping, search, diagnostics."
alwaysApply: true
---

`

const windsurfFrontmatter = `---
trigger: always_on
---

`

// bt is a shorthand for triple backticks to use inside raw string constants.
const bt = "```"

// --- Shared content (frontmatter-free, used by all agents) ---

var zkInstructionContent = `# Zettelkasten Memory CLI (zk)

> AI 에이전트가 지식을 구조화하고 연결하는 CLI 도구.
> A CLI tool for AI agents to structure and connect knowledge.

## When to Use zk

You should **proactively** use zk whenever you:
- Learn something new during a task (create a concrete note)
- Notice a pattern, tension, or contradiction (create an abstract note)
- Make a decision or change your understanding (link with supports/contradicts/replaces/invalidates)
- Finish a research or analysis task (summarize findings as notes)
- Start a new session and need context (search/explore existing notes)

**Do not wait to be asked.** If you are thinking about something worth remembering, record it. If two ideas conflict, link them with contradicts. If new data changes an old conclusion, link with invalidates or replaces. The value of zk comes from habitual use, not occasional use.

## Global Options

` + bt + `bash
--format <fmt>     # Output format: json (default) | yaml | md
--project <id>     # Project scope
--verbose          # Debug output to stderr
--quiet            # Suppress stderr status messages
` + bt + `

## Init & Config

` + bt + `bash
zk init                              # Initialize store
zk init --path /custom               # Custom path
zk config show                       # Show current config
zk config set default_project P-XXX  # Set default project
zk config set default_format yaml    # Set default output format
` + bt + `

## Projects

` + bt + `bash
zk project create <name> --description "desc"
zk project list
zk project get <id>       # Includes note count, link count, last activity
zk project delete <id>
` + bt + `

## Notes (Concrete/Abstract Layers)

Every note belongs to a layer:
- **concrete** (default): facts, observations, data records
- **abstract**: patterns, tensions, questions, insights

` + bt + `bash
# Concrete notes (facts)
zk note create --title "Title" --content "Body" --tags "t1,t2" --layer concrete --project <id>

# With summary (for quick scanning)
zk note create --title "Title" --content "Body" --summary "Brief description" --project <id>

# Abstract notes (insights)
zk note create --title "Tension: X vs Y" --content "..." --layer abstract --project <id>

# Templates
zk note create --title "Title" --template research --project <id>

zk note get <noteID> --project <id>
zk note list --project <id>
zk note list --layer abstract --project <id>     # Filter by layer
zk note update <noteID> --title "New" --project <id>
zk note update <noteID> --summary "Updated summary" --project <id>
zk note delete <noteID> --project <id>           # Blocked if backlinks exist
zk note delete <noteID> --force --project <id>   # Force (moves to trash/)
zk note move <noteID> <targetProject> --project <sourceProject>
` + bt + `

## Links (Relation Type + Weight)

` + bt + `bash
# Same project
zk link add <src> <tgt> --type supports --weight 0.8 --project <id>

# Cross-project
zk link add <src> <tgt> --type extends --project P-1 --target-project P-2

zk link remove <src> <tgt> --project <id>
zk link list <noteID> --project <id>
zk link list <noteID> --type supports              # Filter by relation type
zk link list <noteID> --sort-weight                 # Sort by weight desc
zk link list <noteID> --depth 3 --project <id>     # BFS traversal
` + bt + `

Relation types: related (default), supports, contradicts, extends, causes, example-of, abstracts, grounds, replaces, invalidates
- abstracts: concrete → abstract ("this fact led to this insight")
- grounds: abstract → concrete ("this insight is grounded in this fact")
- replaces: new note supersedes an older one
- invalidates: data disproves a hypothesis

Duplicate links are automatically prevented.
Cross-project backlinks are included in link list results.

## Search

` + bt + `bash
zk search <query> --project <id>
zk search "Redis" --tags "cache" --relation supports --min-weight 0.5
zk search "data" --created-after 2026-01-01 --created-before 2026-12-31
zk search "auth" --sort relevance    # relevance | created | updated
zk search "tension" --layer abstract --project <id>   # Search only abstract notes
` + bt + `

## Tags

` + bt + `bash
zk tag add <noteID> <tag1> [tag2...] --project <id>
zk tag remove <noteID> <tag1> [tag2...]
zk tag replace <oldTag> <newTag> --project <id>
zk tag list --project <id>
zk tag batch-add <tag> <noteID1> [noteID2...]
` + bt + `

## Diagnostics

` + bt + `bash
zk diagnose --project <id>
` + bt + `

Checks: broken links, corrupted files, orphan notes, invalid relation types, out-of-range weights.

## Export & Import

` + bt + `bash
zk export --project <id> --format yaml --output backup.yaml
zk export --project <id> --notes N-AAA,N-BBB
zk import --file backup.yaml --project <id> --conflict skip  # skip|overwrite|new-id
` + bt + `

## Schema Introspection

` + bt + `bash
zk schema              # List all resources
zk schema note         # Note field details
zk schema link         # Link field details
zk schema relation-types
` + bt + `

## Pipeline-Safe Output

- stdout: pure data only (JSON/YAML/Markdown)
- stderr: status, errors, debug info
- Use --quiet to suppress stderr status messages

## Reflect — Insight Engine

Analyzes concrete notes and suggests missing abstract notes:

` + bt + `bash
zk reflect --project <id>              # Show insight suggestions
zk reflect --project <id> --format md  # Markdown report
zk reflect --project <id> --apply      # Auto-create suggested abstract notes
` + bt + `

Detects:
- **Tensions**: contradicts pairs without a synthesizing abstract note
- **Hubs without abstraction**: concrete notes with 4+ links but no abstract
- **Orphan notes**: notes with zero connections
- **Abstraction ratio**: warns if abstract/total ratio is too low

When --apply is used, zk automatically:
1. Creates abstract notes with suggested titles
2. Links them to source concrete notes via "abstracts" relation
3. Tags them with "auto-reflect"

## Graph Visualization

` + bt + `bash
zk graph --project <id>                              # Mermaid graph (default)
zk graph --project <id> --format-graph dot            # DOT format
zk graph --project <id> --layer abstract              # Abstract notes only
zk graph --project <id> --type contradicts            # Only contradiction edges
` + bt + `

Outputs to stdout. Pipe to file: ` + "`zk graph --project P-XXX > graph.mmd`" + `

## Explore — Interactive Navigation

` + bt + `bash
zk explore <noteID> --project <id>                    # Show connections
zk explore <noteID> --project <id> --depth 2          # Include neighbors' neighbors
zk explore <noteID> --project <id> --include-content  # Include note body
zk explore <noteID> --project <id> --format md        # Human-readable with navigation hints
` + bt + `

Use explore to navigate the knowledge graph step by step. The JSON output includes outgoing links, incoming backlinks, and neighbor nodes for deeper traversal.

## Agent Workflows

### 1. Knowledge Accumulation (Concrete)
` + bt + `bash
zk init
zk project create "research" --description "Research project"
zk note create --title "Finding 1" --content "..." --tags "finding" --layer concrete --project P-XXX
zk note create --title "Finding 2" --content "..." --tags "finding" --layer concrete --project P-XXX
zk link add N-AAA N-BBB --type supports --weight 0.9 --project P-XXX
` + bt + `

### 2. Insight Derivation (Concrete → Abstract)
` + bt + `bash
# Check what insights are missing
zk reflect --project P-XXX --format md

# Auto-create abstract notes from analysis
zk reflect --project P-XXX --apply

# Or manually create abstract notes
zk note create --title "Growth vs Retention tradeoff" --content "..." --layer abstract --project P-XXX
zk link add N-CONCRETE N-ABSTRACT --type abstracts --weight 0.8 --project P-XXX
` + bt + `

### 3. Knowledge Exploration
` + bt + `bash
zk search "keyword" --project P-XXX
zk search "tension" --layer abstract --project P-XXX    # Browse insights only
zk link list N-AAA --depth 2 --project P-XXX
zk note get N-BBB --project P-XXX
` + bt + `

### 4. Knowledge Navigation
` + bt + `bash
zk graph --project P-XXX > knowledge-map.mmd          # Generate visual map
zk explore N-HUB --project P-XXX --depth 2             # Start from hub note
zk explore N-NEXT --project P-XXX --include-content    # Dive into connected note
` + bt + `

### 5. Maintenance
` + bt + `bash
zk diagnose --project P-XXX
zk reflect --project P-XXX           # Check abstraction health
zk export --project P-XXX --output snapshot.yaml
` + bt + `

## Storage Layout

` + bt + `
{store_path}/
├── config.yaml
├── projects/{project-id}/
│   ├── project.yaml
│   └── notes/{note-id}.md    # YAML frontmatter + Markdown body
├── global/notes/              # Project-less notes
├── trash/                     # Soft-deleted notes
└── templates/                 # Note templates (.yaml)
` + bt + `

## Key Notes

- Note metadata supports an optional ` + "`summary`" + ` field for quick scanning
- Note files use YAML frontmatter + Markdown body format
- Links are bidirectional (add creates both source→target and target→source)
- Without --project, notes go to global scope
- Deleted notes move to trash/ (recoverable)
- exit code: 0=success, 1=error
`

var domainGuideContent = `# Zettelkasten Domain Guide

> Domain knowledge and best practices for the zk memory CLI.

## Core Principles

### Atomic Notes
- One note = one idea/information unit
- Keep notes focused and reusable
- Split complex topics into connected atomic notes

### Concrete/Abstract Layers
Notes belong to one of two layers:
- **concrete**: Facts, observations, metrics, specifications, data points
- **abstract**: Patterns, tensions, questions, insights, strategic decisions

The power of zk comes from the interplay between layers:
- Concrete notes accumulate raw knowledge
- Abstract notes emerge when you notice patterns, contradictions, or questions across concrete notes
- Use ` + "`zk reflect`" + ` to automatically detect where abstract notes are needed

### Bidirectional Links
- All links are automatically bidirectional
- Relation types express "why" the connection exists
- Weights express "how strong" the connection is

## Relation Type Guide

| Type | Meaning | Layer Direction | Example |
|------|---------|-----------------|---------|
| related | General relation (default) | any → any | Different angles of same topic |
| supports | Evidence, backing | any → any | Evidence supports a claim |
| contradicts | Contradiction | any → any | Conflicting opinions or data |
| extends | Extension | any → any | Develops an idea further |
| causes | Causation | any → any | Cause-effect relationship |
| example-of | Instance | any → any | Concrete example of a concept |
| abstracts | Insight derivation | concrete → abstract | "This fact led to this insight" |
| grounds | Evidence grounding | abstract → concrete | "This insight is based on this fact" |
| replaces | Supersession | any → any | New note supersedes an older one |
| invalidates | Disproof | any → any | Data disproves a hypothesis |

### When to use abstracts/grounds
- After creating several concrete notes, ask: "What pattern or tension do I see?"
- Create an abstract note for the insight
- Link concrete → abstract with "abstracts" relation
- Link abstract → concrete with "grounds" relation
- This makes the reasoning chain explicit and traversable

## Weight Guide

| Range | Meaning | When to use |
|-------|---------|-------------|
| 0.8~1.0 | Very strong (core connection) | Direct evidence, primary cause, key insight |
| 0.5~0.7 | Moderate (reference level) | Supporting context, related but not central |
| 0.1~0.4 | Weak (indirect connection) | Tangential, might be relevant later |

## Best Practices

1. **Start concrete, derive abstract**: Record facts first, then notice patterns
2. **Use ` + "`zk reflect`" + ` regularly**: It detects tensions, orphans, and hubs that need abstraction
3. **Name abstract notes as questions**: "X vs Y — what should we choose?" is more useful than "Analysis of X"
4. **Isolate context with projects**: Group related notes in the same project
5. **Cross-cut with tags**: Use tags for themes that span projects
6. **Run ` + "`zk diagnose`" + ` periodically**: Find broken links and orphan notes
7. **Backup with ` + "`zk export`" + `**: Regular snapshots prevent data loss
8. **Use specific relation types**: Don't just use "related" — express the actual relationship
9. **Leverage search filters**: Combine --tags, --relation, --min-weight, --layer for precise queries
10. **Track abstraction ratio**: Aim for 20-30% abstract notes; below 10% suggests missing insights
11. **Write summaries for long notes**: Notes over 500 chars benefit from a summary — agents scan summaries before reading full content

## Proactive Usage Mindset

zk is not a passive archive — it is an active thinking tool. As an AI agent, you should:
- **Record as you go**: When you discover a fact, make a decision, or change your mind — write it down immediately
- **Link aggressively**: Every note should connect to at least one other note. If it doesn't, ask why it exists in isolation
- **Challenge yourself**: After creating 3+ concrete notes, run ` + "`zk reflect`" + ` and act on the suggestions
- **Use contradicts liberally**: Disagreement and tension are the most productive signals — they show where thinking is needed
- **Update, don't append**: If a note's conclusion changes, create a new note and link with replaces/invalidates rather than making the old note longer
- **Summarize proactively**: When creating or updating a note with >500 chars, add a --summary so future sessions can scan quickly

## Anti-Patterns

- **Waiting to be told**: Don't wait for the user to say "record this" — if it's worth thinking about, it's worth noting
- **Dumping without linking**: Creating notes without connecting them defeats the purpose
- **All concrete, no abstract**: Recording facts without deriving insights means no structured thinking
- **Vague relations**: Using "related" for everything loses the semantic richness
- **Ignoring tensions**: contradicts links are the most valuable — they reveal where decisions are needed
- **Appending endlessly**: Growing a single note instead of splitting into hypothesis → evidence → conclusion
`
