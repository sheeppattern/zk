package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/nete/internal/model"
)

// SchemaField describes a single field in a resource schema.
type SchemaField struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Required    bool   `json:"required" yaml:"required"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description" yaml:"description"`
}

// SchemaResource describes a resource and its fields.
type SchemaResource struct {
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description" yaml:"description"`
	Fields      []SchemaField `json:"fields,omitempty" yaml:"fields,omitempty"`
}

// buildSchemaRegistry returns all known resource schemas.
func buildSchemaRegistry() []SchemaResource {
	relationValues := strings.Join(model.ValidRelationTypes(), ", ")

	return []SchemaResource{
		{
			Name:        "memo",
			Description: "An atomic memory record in the Zettelkasten",
			Fields: []SchemaField{
				{Name: "id", Type: "int64", Required: false, Description: "Auto-generated unique identifier"},
				{Name: "title", Type: "string", Required: true, Description: "Title of the memo"},
				{Name: "content", Type: "string", Required: false, Description: "Body content of the memo"},
				{Name: "tags", Type: "string[]", Required: false, Description: "List of tags for categorization"},
				{Name: "layer", Type: "string", Required: false, Default: "concrete", Description: "Layer: concrete or abstract"},
				{Name: "note_id", Type: "int64", Required: false, Default: "0", Description: "ID of the note this memo belongs to (0 = global)"},
				{Name: "metadata", Type: "Metadata", Required: false, Description: "Auto-managed metadata object"},
			},
		},
		{
			Name:        "note",
			Description: "A grouping that holds related memos together",
			Fields: []SchemaField{
				{Name: "id", Type: "int64", Required: false, Description: "Auto-generated unique identifier"},
				{Name: "name", Type: "string", Required: true, Description: "Name of the note"},
				{Name: "description", Type: "string", Required: false, Description: "Description of the note"},
				{Name: "created_at", Type: "datetime", Required: false, Description: "Timestamp when the note was created (auto-set)"},
				{Name: "updated_at", Type: "datetime", Required: false, Description: "Timestamp when the note was last updated (auto-set)"},
			},
		},
		{
			Name:        "link",
			Description: "A weighted, typed connection between memos",
			Fields: []SchemaField{
				{Name: "source_id", Type: "int64", Required: true, Description: "ID of the source memo"},
				{Name: "target_id", Type: "int64", Required: true, Description: "ID of the target memo"},
				{Name: "relation_type", Type: "string", Required: false, Default: "related", Description: fmt.Sprintf("Type of relation; valid values: %s", relationValues)},
				{Name: "weight", Type: "float64", Required: false, Default: "0.5", Description: "Connection strength from 0.0 to 1.0"},
			},
		},
		{
			Name:        "metadata",
			Description: "Auto-recorded metadata for a memo",
			Fields: []SchemaField{
				{Name: "created_at", Type: "datetime", Required: false, Description: "Timestamp when the memo was created (auto-set)"},
				{Name: "updated_at", Type: "datetime", Required: false, Description: "Timestamp when the memo was last updated (auto-set)"},
				{Name: "source", Type: "string", Required: false, Description: "Origin or source of the memo content"},
				{Name: "status", Type: "string", Required: false, Default: "active", Description: "Memo status: active or archived"},
				{Name: "summary", Type: "string", Required: false, Description: "Brief summary for quick scanning"},
				{Name: "author", Type: "string", Required: false, Description: "Author of the memo"},
			},
		},
		{
			Name:        "config",
			Description: "CLI configuration settings",
			Fields: []SchemaField{
				{Name: "store_path", Type: "string", Required: false, Description: "Path to the data store directory"},
				{Name: "default_note", Type: "string", Required: false, Description: "Default note scope for operations"},
				{Name: "default_format", Type: "string", Required: false, Description: "Default output format: json, yaml, or md"},
				{Name: "default_author", Type: "string", Required: false, Description: "Default author for new memos"},
			},
		},
		{
			Name:        "relation-types",
			Description: "All valid relation types for links between memos",
			Fields: buildRelationTypeFields(),
		},
	}
}

// buildRelationTypeFields generates a SchemaField per valid relation type.
func buildRelationTypeFields() []SchemaField {
	descriptions := map[string]string{
		"related":     "General association between memos",
		"supports":    "Source memo provides evidence or support for target",
		"contradicts": "Source memo contradicts or conflicts with target",
		"extends":     "Source memo extends or elaborates on target",
		"causes":      "Source memo describes a cause of the target",
		"example-of":  "Source memo is an example of the concept in target",
		"abstracts":   "Concrete memo led to this abstract insight",
		"grounds":     "Abstract insight is grounded in this concrete evidence",
		"replaces":    "New memo supersedes an older one",
		"invalidates": "Data disproves a hypothesis",
	}

	types := model.ValidRelationTypes()
	fields := make([]SchemaField, len(types))
	for i, rt := range types {
		desc := descriptions[rt]
		if desc == "" {
			desc = "A valid relation type"
		}
		fields[i] = SchemaField{
			Name:        rt,
			Type:        "relation",
			Required:    false,
			Description: desc,
		}
	}
	return fields
}

// findResource looks up a resource by name in the registry.
func findResource(name string, registry []SchemaResource) *SchemaResource {
	for i := range registry {
		if registry[i].Name == name {
			return &registry[i]
		}
	}
	return nil
}

// printSchemaListMD prints a markdown list of all resources (without fields).
func printSchemaListMD(resources []SchemaResource) {
	var b strings.Builder
	fmt.Fprintln(&b, "# Available Resources")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Resource | Description |")
	fmt.Fprintln(&b, "|----------|-------------|")
	for _, r := range resources {
		fmt.Fprintf(&b, "| %s | %s |\n", r.Name, r.Description)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Run `nete schema <resource>` for detailed field information.")
	fmt.Fprint(os.Stdout, b.String())
}

// printSchemaDetailMD prints a markdown detail view of a single resource.
func printSchemaDetailMD(r *SchemaResource) {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", r.Name)
	fmt.Fprintf(&b, "%s\n\n", r.Description)
	fmt.Fprintln(&b, "| Field | Type | Required | Default | Description |")
	fmt.Fprintln(&b, "|-------|------|----------|---------|-------------|")
	for _, f := range r.Fields {
		req := "no"
		if f.Required {
			req = "yes"
		}
		def := f.Default
		if def == "" {
			def = "-"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", f.Name, f.Type, req, def, f.Description)
	}
	fmt.Fprint(os.Stdout, b.String())
}

var schemaCmd = &cobra.Command{
	Use:   "schema [resource]",
	Short: "Show schema information for resources",
	Long:  "Display available resources and their field definitions.",
	Example: `  nete schema
  nete schema memo
  nete schema relation-types`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := buildSchemaRegistry()
		f := getFormatter()

		if len(args) == 0 {
			summaries := make([]SchemaResource, len(registry))
			for i, r := range registry {
				summaries[i] = SchemaResource{
					Name:        r.Name,
					Description: r.Description,
				}
			}

			switch f.Format {
			case "json":
				return f.PrintJSON(summaries)
			case "yaml":
				return f.PrintYAML(summaries)
			default:
				printSchemaListMD(summaries)
				return nil
			}
		}

		name := args[0]
		resource := findResource(name, registry)
		if resource == nil {
			names := make([]string, len(registry))
			for i, r := range registry {
				names[i] = r.Name
			}
			return fmt.Errorf("unknown resource %q; available: %s", name, strings.Join(names, ", "))
		}

		switch f.Format {
		case "json":
			return f.PrintJSON(resource)
		case "yaml":
			return f.PrintYAML(resource)
		default:
			printSchemaDetailMD(resource)
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
