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
	Short: "Manage nete skill definitions for AI coding agents",
}

var skillGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate instruction files for AI coding agents",
	Long: `Generate instruction/skill files for multiple AI coding agents.

Global files (Claude, Gemini, Codex) are always generated at ~/.
Project files (Cursor, Copilot, Windsurf) require --project-dir.`,
	Example: `  nete skill generate
  nete skill generate --project-dir .
  nete skill generate --agents claude,cursor --project-dir .
  nete skill generate --global-only`,
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

// Claude Code: ~/.claude/skills/nete/SKILL.md + references/domain-guide.md
func writeClaudeSkill(home string) (string, error) {
	dir := filepath.Join(home, ".claude", "skills", "nete")
	skillPath := filepath.Join(dir, "SKILL.md")

	content := claudeFrontmatter + neteInstructionContent
	if err := writeFile(skillPath, content); err != nil {
		return "", err
	}

	domainPath := filepath.Join(dir, "references", "domain-guide.md")
	if err := writeFile(domainPath, domainGuideContent); err != nil {
		return "", err
	}

	return skillPath, nil
}

// Gemini CLI: ~/.gemini/instructions/nete.md
func writeGeminiInstruction(home string) (string, error) {
	path := filepath.Join(home, ".gemini", "instructions", "nete.md")
	if err := writeFile(path, neteInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Codex CLI: ~/.codex/instructions/nete.md
func writeCodexInstruction(home string) (string, error) {
	path := filepath.Join(home, ".codex", "instructions", "nete.md")
	if err := writeFile(path, neteInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Agent Skills Standard (agentskills.io): ~/.agents/skills/nete/SKILL.md (global)
func writeAgentSkillsGlobal(home string) (string, error) {
	return writeAgentSkillsDir(filepath.Join(home, ".agents", "skills", "nete"))
}

// Agent Skills Standard (agentskills.io): {projectDir}/.agents/skills/nete/SKILL.md (project)
func writeAgentSkillsProject(projectDir string) (string, error) {
	return writeAgentSkillsDir(filepath.Join(projectDir, ".agents", "skills", "nete"))
}

func writeAgentSkillsDir(dir string) (string, error) {
	skillPath := filepath.Join(dir, "SKILL.md")

	content := claudeFrontmatter + neteInstructionContent
	if err := writeFile(skillPath, content); err != nil {
		return "", err
	}

	domainPath := filepath.Join(dir, "references", "domain-guide.md")
	if err := writeFile(domainPath, domainGuideContent); err != nil {
		return "", err
	}

	return skillPath, nil
}

// Cursor: {projectDir}/.cursor/rules/nete.mdc
func writeCursorRule(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".cursor", "rules", "nete.mdc")
	content := cursorFrontmatter + neteInstructionContent
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// GitHub Copilot: {projectDir}/.github/copilot-instructions.md
func writeCopilotInstruction(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".github", "copilot-instructions.md")
	if err := writeFile(path, neteInstructionContent); err != nil {
		return "", err
	}
	return path, nil
}

// Windsurf: {projectDir}/.windsurf/rules/nete.md
func writeWindsurfRule(projectDir string) (string, error) {
	path := filepath.Join(projectDir, ".windsurf", "rules", "nete.md")
	content := windsurfFrontmatter + neteInstructionContent
	if err := writeFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// --- Frontmatter constants ---

const claudeFrontmatter = `---
name: nete
description: "Zettelkasten memory CLI — AI 에이전트용 지식 관리 도구. SQLite + FTS5 풀텍스트 검색, 메모 CRUD, 관계 타입+가중치 링크, 노트 범위 관리, 웹 GUI를 지원합니다."
---

`

const cursorFrontmatter = `---
description: "nete - Zettelkasten memory CLI for AI agents. SQLite + FTS5 search, memo CRUD, typed+weighted links, note scoping, web GUI."
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

var neteInstructionContent = `# Zettelkasten Memory CLI (nete)

> AI 에이전트가 지식을 구조화하고 연결하는 CLI 도구.
> A CLI tool for AI agents to structure and connect knowledge.

## When to Use nete

You should **proactively** use nete whenever you:
- Learn something new during a task (create a concrete memo)
- Notice a pattern, tension, or contradiction (create an abstract memo)
- Make a decision or change your understanding (link with supports/contradicts/replaces/invalidates)
- Finish a research or analysis task (summarize findings as memos)
- Start a new session and need context (search/explore existing memos)

**Do not wait to be asked.** If you are thinking about something worth remembering, record it. The value of nete comes from habitual use, not occasional use.

## Concepts

- **Note**: a container that groups related memos (like a folder/project)
- **Memo**: an atomic knowledge record (the actual content)
- **Link**: a typed, weighted connection between memos (single-stored, queried both ways)
- IDs are integers (1, 2, 3...), auto-incremented by the database

## Global Options

` + bt + `bash
--format <fmt>     # Output format: json (default) | yaml | md
--note <id>        # Note scope for memos (0 = global)
--verbose          # Debug output to stderr
--quiet            # Suppress stderr status messages
` + bt + `

## Init & Config

` + bt + `bash
nete init                              # Initialize store (SQLite)
nete init --path /custom               # Custom path
nete config show                       # Show current config
nete config set default_note 1         # Set default note scope
nete config set default_format yaml    # Set default output format
nete config set default_author claude  # Set default memo author
` + bt + `

## Notes (Containers)

` + bt + `bash
nete note create <name> --description "desc"
nete note list
nete note get <id>       # Includes memo count, link count
nete note delete <id>
` + bt + `

## Memos (Concrete/Abstract Layers)

Every memo belongs to a layer:
- **concrete** (default): facts, observations, data records
- **abstract**: patterns, tensions, questions, insights

` + bt + `bash
# Concrete memos (facts)
nete memo create --title "Title" --content "Body" --tags "t1,t2" --layer concrete --note <id>

# With summary and source
nete memo create --title "Title" --content "Body" --summary "Brief" --note <id>

# Abstract memos (insights)
nete memo create --title "Tension: X vs Y" --content "..." --layer abstract --note <id>

nete memo get <memoID>
nete memo list --note <id>
nete memo list --layer abstract --note <id>
nete memo update <memoID> --title "New"
nete memo update <memoID> --summary "Updated summary"
nete memo delete <memoID>
nete memo move <memoID> <targetNoteID>
nete memo random                        # Random memo from all notes
nete memo random --layer abstract       # Random abstract memo

# Author tracking
nete memo create --title "Title" --content "..." --author claude --note <id>
` + bt + `

## Quick Memo

Minimal memo creation from a single text argument:

` + bt + `bash
nete quickmemo "My quick thought here"
nete quickmemo "Observation about X" --note <id>
nete quickmemo "Found a pattern" --author claude
` + bt + `

- Title: auto-derived (first 50 chars, truncated at word boundary)
- Content: full input text
- Layer: concrete (default)

## Links (Relation Type + Weight)

Links are stored once and queried both directions (no bidirectional duplication).

` + bt + `bash
nete link add <src> <tgt> --type supports --weight 0.8
nete link remove <src> <tgt> --type supports
nete link list <memoID>                              # Show outgoing + incoming
nete link list <memoID> --type supports              # Filter by relation type
nete link list <memoID> --sort-weight                # Sort by weight desc
nete link list <memoID> --depth 3                    # BFS traversal (max depth 5)
` + bt + `

Relation types: related (default), supports, contradicts, extends, causes, example-of, abstracts, grounds, replaces, invalidates

## Search (FTS5 Full-Text)

Powered by SQLite FTS5 with BM25 ranking. Searches title, content, tags, and summary.

` + bt + `bash
nete search <query>
nete search "Redis" --tags "cache"
nete search "auth" --sort relevance                  # relevance | created | updated
nete search "tension" --layer abstract --note <id>
nete search "pattern" --author claude
nete search "data" --created-after 2026-01-01 --created-before 2026-12-31
` + bt + `

FTS5 syntax: wrap in quotes for phrase match. Prefix matching with *.

## Tags

` + bt + `bash
nete tag add <memoID> <tag1> [tag2...]
nete tag remove <memoID> <tag1> [tag2...]
nete tag replace <oldTag> <newTag> --note <id>
nete tag list --note <id>
nete tag batch-add <tag> <memoID1> [memoID2...]
` + bt + `

## Diagnostics

` + bt + `bash
nete diagnose
nete diagnose --format md
` + bt + `

Checks: orphan memos, invalid relation types, out-of-range weights.

## Export & Import

` + bt + `bash
nete export --note <id> --format yaml --output backup.yaml
nete import --file backup.yaml --note <id>
` + bt + `

## Reflect — Insight Engine

` + bt + `bash
nete reflect --note <id>                 # Show insight suggestions
nete reflect --note <id> --format md     # Markdown report
nete reflect --note <id> --apply         # Auto-create suggested abstract memos
nete reflect --note <id> --suggest-links # Suggest missing links
` + bt + `

Detects: tensions, hubs without abstraction, orphan memos, low abstraction ratio, similar unlinked memos.

## Graph & Explore

` + bt + `bash
nete graph --note <id>                               # Mermaid graph
nete graph --note <id> --format-graph dot            # DOT format
nete explore <memoID> --depth 2                      # Show connections
nete explore <memoID> --include-content --format md  # Full detail
` + bt + `

## Web GUI

` + bt + `bash
nete serve                               # http://127.0.0.1:8080
nete serve --addr :3000                  # Custom port
` + bt + `

Features: memo editor (title, summary, content, tags, status, source), incoming/outgoing link panels, neighborhood graph minimap, FTS5 search, note/memo tree explorer.

## Schema Introspection

` + bt + `bash
nete schema              # List all resources
nete schema memo         # Memo field details
nete schema link         # Link field details
nete schema relation-types
` + bt + `

## Agent Workflows

### 1. Knowledge Accumulation
` + bt + `bash
nete init
nete note create "research" --description "Research project"
nete memo create --title "Finding 1" --content "..." --tags "finding" --note 1
nete memo create --title "Finding 2" --content "..." --tags "finding" --note 1
nete link add 1 2 --type supports --weight 0.9
` + bt + `

### 2. Insight Derivation
` + bt + `bash
nete reflect --note 1 --format md       # Check what insights are missing
nete reflect --note 1 --apply           # Auto-create abstract memos
nete memo create --title "Growth vs Retention" --content "..." --layer abstract --note 1
nete link add 1 3 --type abstracts --weight 0.8
` + bt + `

### 3. Knowledge Exploration
` + bt + `bash
nete search "keyword" --note 1
nete search "tension" --layer abstract
nete link list 1 --depth 2
nete memo get 2
` + bt + `

### 4. Maintenance
` + bt + `bash
nete diagnose
nete reflect --note 1
nete export --note 1 --output snapshot.yaml
` + bt + `

### 5. Serendipity — Cross-Pollination Discovery
` + bt + `bash
# Step 1: Pick 2–5 random memos (repeat nete memo random multiple times)
nete memo random --format json
nete memo random --format json
nete memo random --format json

# Step 2: Read each memo's full content
nete memo get <id1> --format md
nete memo get <id2> --format md
nete memo get <id3> --format md

# Step 3: Analyze & link (you, the agent, do this)
# - Find non-obvious connections between the random memos
# - Propose relation type and weight for each connection
# - Delegate logical validation to a sub-agent if available; otherwise self-review
# - Create links for validated connections only

nete link add <id1> <id2> --type related --weight 0.6
nete memo create --title "Serendipity: X connects to Y" \
  --content "Found via random exploration: ..." --layer abstract
` + bt + `

**Serendipity workflow rules:**
- Pick 2–5 memos randomly across ALL notes (not limited to one note)
- Look for hidden patterns, analogies, contradictions, or causal chains
- Use a sub-agent (if available) to verify logical coherence before creating links
- If no sub-agent, self-review: ask "Would a skeptic accept this connection?"
- Only create links when the connection is defensible — skip forced associations
- Tag serendipity-born memos with ` + "`serendipity`" + ` for traceability

## Storage

` + bt + `
{store_path}/
└── store.db     # Single SQLite database (FTS5, WAL mode)
` + bt + `

Tables: notes, memos, memos_fts (FTS5), links, trash, config.

## Key Facts

- Storage: single SQLite file (store.db) with FTS5 full-text search
- IDs: integer auto-increment (1, 2, 3...)
- Links: single-stored, queried both directions (no duplication)
- Memos support: title, content, tags, layer, summary, author, source, status
- Use ` + "`nete quickmemo`" + ` for fast capture
- Without --note, memos go to global scope (note_id=0)
- Deleted memos move to trash (recoverable)
- Pipeline-safe: stdout = data, stderr = status/errors
- exit code: 0=success, 1=error
`

var domainGuideContent = `# Zettelkasten Domain Guide

> Domain knowledge and best practices for the nete memory CLI.

## Core Principles

### Atomic Memos
- One memo = one idea/information unit
- Keep memos focused and reusable
- Split complex topics into connected atomic memos

### Concrete/Abstract Layers
Memos belong to one of two layers:
- **concrete**: Facts, observations, metrics, specifications, data points
- **abstract**: Patterns, tensions, questions, insights, strategic decisions

The power of nete comes from the interplay between layers:
- Concrete memos accumulate raw knowledge
- Abstract memos emerge when you notice patterns, contradictions, or questions
- Use ` + "`nete reflect`" + ` to automatically detect where abstract memos are needed

### Links
- Links are stored once and queried both directions
- Relation types express "why" the connection exists
- Weights express "how strong" the connection is (0.0–1.0)

## Relation Type Guide

| Type | Meaning | Example |
|------|---------|---------|
| related | General relation (default) | Different angles of same topic |
| supports | Evidence, backing | Evidence supports a claim |
| contradicts | Contradiction | Conflicting opinions or data |
| extends | Extension | Develops an idea further |
| causes | Causation | Cause-effect relationship |
| example-of | Instance | Concrete example of a concept |
| abstracts | Insight derivation | "This fact led to this insight" |
| grounds | Evidence grounding | "This insight is based on this fact" |
| replaces | Supersession | New memo supersedes an older one |
| invalidates | Disproof | Data disproves a hypothesis |

## Weight Guide

| Range | Meaning | When to use |
|-------|---------|-------------|
| 0.8–1.0 | Very strong (core) | Direct evidence, primary cause, key insight |
| 0.5–0.7 | Moderate (reference) | Supporting context, related but not central |
| 0.1–0.4 | Weak (indirect) | Tangential, might be relevant later |

## Best Practices

1. **Start concrete, derive abstract**: Record facts first, then notice patterns
2. **Use ` + "`nete reflect`" + ` regularly**: Detects tensions, orphans, and hubs needing abstraction
3. **Name abstract memos as questions**: "X vs Y — what should we choose?"
4. **Isolate context with notes**: Group related memos in the same note
5. **Cross-cut with tags**: Use tags for themes that span notes
6. **Run ` + "`nete diagnose`" + ` periodically**: Find orphan memos and invalid links
7. **Backup with ` + "`nete export`" + `**: Regular snapshots prevent data loss
8. **Use specific relation types**: Don't just use "related" — express the actual relationship
9. **Leverage FTS5 search**: Combine --tags, --layer, --author for precise queries
10. **Write summaries**: Memos over 500 chars benefit from a --summary for quick scanning
11. **Use ` + "`nete quickmemo`" + `**: Fast capture when structure can wait

## Proactive Usage Mindset

nete is not a passive archive — it is an active thinking tool. As an AI agent:
- **Record as you go**: Discover → write it down immediately
- **Link aggressively**: Every memo should connect to at least one other
- **Challenge yourself**: After 3+ concrete memos, run ` + "`nete reflect`" + `
- **Use contradicts liberally**: Tension signals where thinking is needed
- **Update, don't append**: New conclusion? New memo + replaces/invalidates link
- **Summarize proactively**: >500 chars → add --summary
- **Set your author**: ` + "`nete config set default_author <name>`" + `
- **Discover hidden links**: ` + "`nete reflect --suggest-links`" + ` periodically

## Anti-Patterns

- **Waiting to be told**: If it's worth thinking about, it's worth noting
- **Dumping without linking**: Unlinked memos defeat the purpose
- **All concrete, no abstract**: Facts without insights = no structured thinking
- **Vague relations**: "related" for everything loses semantic richness
- **Ignoring tensions**: contradicts links are the most valuable
- **Appending endlessly**: Split into hypothesis → evidence → conclusion
`
