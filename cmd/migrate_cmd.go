package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/model"
	"gopkg.in/yaml.v3"
)

// oldProject mirrors the old Project YAML format.
type oldProject struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	CreatedAt   time.Time `yaml:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"`
}

// oldNoteFrontmatter mirrors the old Note YAML frontmatter format.
type oldNoteFrontmatter struct {
	ID        string    `yaml:"id"`
	Title     string    `yaml:"title"`
	Tags      []string  `yaml:"tags"`
	Links     []oldLink `yaml:"links"`
	Metadata  oldMeta   `yaml:"metadata"`
	ProjectID string    `yaml:"project_id"`
	Layer     string    `yaml:"layer"`
}

type oldLink struct {
	TargetID     string  `yaml:"target_id"`
	RelationType string  `yaml:"relation_type"`
	Weight       float64 `yaml:"weight"`
}

type oldMeta struct {
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
	Source    string    `yaml:"source"`
	Status    string    `yaml:"status"`
	Summary   string    `yaml:"summary"`
	Author    string    `yaml:"author"`
}

var migrateCmd = &cobra.Command{
	Use:   "migrate <old-store-path>",
	Short: "Migrate old .md file store to new SQLite format",
	Long:  "Reads projects and notes from an old file-based zk store and imports them into the new SQLite store.",
	Example: `  zk migrate ~/.zk-memory
  zk migrate ~/.zk-memory --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runMigrate,
}

func init() {
	migrateCmd.Flags().Bool("dry-run", false, "preview migration without writing")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	oldPath := args[0]
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Verify old store exists.
	if _, err := os.Stat(filepath.Join(oldPath, "config.yaml")); err != nil {
		return fmt.Errorf("old store not found at %s: %w", oldPath, err)
	}

	// Read old projects.
	var oldProjects []oldProject
	projDir := filepath.Join(oldPath, "projects")
	projEntries, _ := os.ReadDir(projDir)
	for _, e := range projEntries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(projDir, e.Name(), "project.yaml"))
		if err != nil {
			statusf("warning: skipping project %s: %v", e.Name(), err)
			continue
		}
		var p oldProject
		if err := yaml.Unmarshal(data, &p); err != nil {
			statusf("warning: skipping project %s: %v", e.Name(), err)
			continue
		}
		oldProjects = append(oldProjects, p)
	}

	// Read old notes from each project + global.
	type oldNote struct {
		Frontmatter oldNoteFrontmatter
		Content     string
		ProjectID   string // old P-XXXXXX
	}
	var oldNotes []oldNote

	readNotes := func(notesDir, projectID string) {
		entries, _ := os.ReadDir(notesDir)
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(notesDir, e.Name()))
			if err != nil {
				statusf("warning: skipping %s: %v", e.Name(), err)
				continue
			}
			fm, content, err := parseOldNote(data)
			if err != nil {
				statusf("warning: skipping %s: %v", e.Name(), err)
				continue
			}
			oldNotes = append(oldNotes, oldNote{
				Frontmatter: fm,
				Content:     content,
				ProjectID:   projectID,
			})
		}
	}

	for _, p := range oldProjects {
		notesDir := filepath.Join(projDir, p.ID, "notes")
		readNotes(notesDir, p.ID)
	}
	readNotes(filepath.Join(oldPath, "global", "notes"), "")

	statusf("found %d projects, %d notes", len(oldProjects), len(oldNotes))

	if dryRun {
		for _, p := range oldProjects {
			statusf("  project: %s (%s)", p.Name, p.ID)
		}
		for _, n := range oldNotes {
			statusf("  note: %s (%s) project=%s links=%d", n.Frontmatter.Title, n.Frontmatter.ID, n.ProjectID, len(n.Frontmatter.Links))
		}
		statusf("dry-run complete, no changes made")
		return nil
	}

	// Open new store.
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	defer s.Close()

	// Map old project ID → new note ID.
	projectMap := make(map[string]int64) // old P-XXXXXX → new int64
	for _, p := range oldProjects {
		n, err := s.CreateNote(p.Name, p.Description)
		if err != nil {
			return fmt.Errorf("create note for project %s: %w", p.ID, err)
		}
		projectMap[p.ID] = n.ID
		statusf("migrated project %s → note %d (%s)", p.ID, n.ID, p.Name)
	}

	// Map old note ID → new memo ID.
	noteMap := make(map[string]int64) // old N-XXXXXX → new int64
	for _, n := range oldNotes {
		noteID := int64(0)
		if n.ProjectID != "" {
			noteID = projectMap[n.ProjectID]
		}

		tags := n.Frontmatter.Tags
		if tags == nil {
			tags = []string{}
		}

		memo := &model.Memo{
			Title:   n.Frontmatter.Title,
			Content: n.Content,
			Tags:    tags,
			Layer:   n.Frontmatter.Layer,
			NoteID:  noteID,
			Metadata: model.Metadata{
				CreatedAt: n.Frontmatter.Metadata.CreatedAt,
				UpdatedAt: n.Frontmatter.Metadata.UpdatedAt,
				Source:    n.Frontmatter.Metadata.Source,
				Status:    n.Frontmatter.Metadata.Status,
				Summary:   n.Frontmatter.Metadata.Summary,
				Author:    n.Frontmatter.Metadata.Author,
			},
		}
		if memo.Layer == "" {
			memo.Layer = model.LayerConcrete
		}
		if memo.Metadata.Status == "" {
			memo.Metadata.Status = model.StatusActive
		}

		if err := s.CreateMemo(memo); err != nil {
			return fmt.Errorf("create memo for note %s: %w", n.Frontmatter.ID, err)
		}
		noteMap[n.Frontmatter.ID] = memo.ID
	}
	statusf("migrated %d notes → memos", len(oldNotes))

	// Migrate links (deduplicate: only add if both source and target exist).
	linkCount := 0
	seen := make(map[string]bool)
	for _, n := range oldNotes {
		srcID, ok := noteMap[n.Frontmatter.ID]
		if !ok {
			continue
		}
		for _, l := range n.Frontmatter.Links {
			tgtID, ok := noteMap[l.TargetID]
			if !ok {
				debugf("skipping link %s→%s: target not found", n.Frontmatter.ID, l.TargetID)
				continue
			}
			// Deduplicate: old store has bidirectional links (A→B and B→A).
			// New store is single-direction. Use canonical key (smaller first).
			key := fmt.Sprintf("%d→%d:%s", srcID, tgtID, l.RelationType)
			reverseKey := fmt.Sprintf("%d→%d:%s", tgtID, srcID, l.RelationType)
			if seen[key] || seen[reverseKey] {
				continue
			}
			seen[key] = true

			weight := l.Weight
			if weight <= 0 || weight > 1 {
				weight = 0.5
			}
			if err := s.AddLink(srcID, tgtID, l.RelationType, weight); err != nil {
				debugf("link %d→%d (%s): %v", srcID, tgtID, l.RelationType, err)
				continue
			}
			linkCount++
		}
	}
	statusf("migrated %d unique links (deduplicated from bidirectional)", linkCount)

	// Migrate config.
	oldCfgData, err := os.ReadFile(filepath.Join(oldPath, "config.yaml"))
	if err == nil {
		var oldCfg struct {
			DefaultAuthor string `yaml:"default_author"`
			DefaultFormat string `yaml:"default_format"`
		}
		if yaml.Unmarshal(oldCfgData, &oldCfg) == nil {
			cfg, _ := s.LoadConfig()
			if cfg == nil {
				cfg = model.DefaultConfig()
			}
			if oldCfg.DefaultAuthor != "" {
				cfg.DefaultAuthor = oldCfg.DefaultAuthor
			}
			if oldCfg.DefaultFormat != "" {
				cfg.DefaultFormat = oldCfg.DefaultFormat
			}
			_ = s.SaveConfig(cfg)
			statusf("migrated config (author=%s, format=%s)", cfg.DefaultAuthor, cfg.DefaultFormat)
		}
	}

	statusf("migration complete: %d notes, %d memos, %d links", len(oldProjects), len(oldNotes), linkCount)
	return nil
}

func parseOldNote(data []byte) (oldNoteFrontmatter, string, error) {
	s := string(data)

	// Split on "---\n" delimiters.
	if !strings.HasPrefix(s, "---\n") {
		return oldNoteFrontmatter{}, "", fmt.Errorf("missing frontmatter delimiter")
	}
	rest := s[4:] // skip first "---\n"
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// Try "---\n" at the very end.
		idx = strings.Index(rest, "\n---")
		if idx < 0 {
			return oldNoteFrontmatter{}, "", fmt.Errorf("missing closing frontmatter delimiter")
		}
	}

	yamlPart := rest[:idx]
	content := ""
	if idx+5 < len(rest) {
		content = strings.TrimSpace(rest[idx+5:])
	}

	var fm oldNoteFrontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return oldNoteFrontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, content, nil
}
