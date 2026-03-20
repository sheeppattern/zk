package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ymh/zk/internal/model"
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
			Name:        "note",
			Description: "An atomic memory note in the Zettelkasten",
			Fields: []SchemaField{
				{Name: "id", Type: "string", Required: false, Description: "Auto-generated unique identifier (e.g. N-ABCDEF)"},
				{Name: "title", Type: "string", Required: true, Description: "Title of the note"},
				{Name: "content", Type: "string", Required: true, Description: "Body content of the note"},
				{Name: "tags", Type: "string[]", Required: false, Description: "List of tags for categorization"},
				{Name: "links", Type: "Link[]", Required: false, Description: "Array of Link objects connecting to other notes"},
				{Name: "metadata", Type: "Metadata", Required: false, Description: "Auto-managed metadata object"},
				{Name: "project_id", Type: "string", Required: false, Description: "ID of the project this note belongs to"},
			},
		},
		{
			Name:        "link",
			Description: "A weighted, typed connection between notes",
			Fields: []SchemaField{
				{Name: "target_id", Type: "string", Required: true, Description: "ID of the target note"},
				{Name: "relation_type", Type: "string", Required: false, Default: "related", Description: fmt.Sprintf("Type of relation; valid values: %s", relationValues)},
				{Name: "weight", Type: "float64", Required: false, Default: "0.5", Description: "Connection strength from 0.0 to 1.0"},
			},
		},
		{
			Name:        "metadata",
			Description: "Auto-recorded metadata for a note",
			Fields: []SchemaField{
				{Name: "created_at", Type: "datetime", Required: false, Description: "Timestamp when the note was created (auto-set)"},
				{Name: "updated_at", Type: "datetime", Required: false, Description: "Timestamp when the note was last updated (auto-set)"},
				{Name: "source", Type: "string", Required: false, Description: "Origin or source of the note content"},
				{Name: "status", Type: "string", Required: false, Default: "active", Description: "Note status: active or archived"},
			},
		},
		{
			Name:        "project",
			Description: "A project that groups related notes together",
			Fields: []SchemaField{
				{Name: "id", Type: "string", Required: false, Description: "Auto-generated unique identifier (e.g. P-ABCDEF)"},
				{Name: "name", Type: "string", Required: true, Description: "Name of the project"},
				{Name: "description", Type: "string", Required: false, Description: "Description of the project"},
				{Name: "created_at", Type: "datetime", Required: false, Description: "Timestamp when the project was created (auto-set)"},
				{Name: "updated_at", Type: "datetime", Required: false, Description: "Timestamp when the project was last updated (auto-set)"},
			},
		},
		{
			Name:        "config",
			Description: "CLI configuration settings",
			Fields: []SchemaField{
				{Name: "store_path", Type: "string", Required: false, Description: "Path to the data store directory"},
				{Name: "default_project", Type: "string", Required: false, Description: "Default project scope for operations"},
				{Name: "default_format", Type: "string", Required: false, Description: "Default output format: json, yaml, or md"},
			},
		},
		{
			Name:        "relation-types",
			Description: "All valid relation types for links between notes",
			Fields: buildRelationTypeFields(),
		},
	}
}

// buildRelationTypeFields generates a SchemaField per valid relation type.
func buildRelationTypeFields() []SchemaField {
	descriptions := map[string]string{
		"related":     "General association between notes",
		"supports":    "Source note provides evidence or support for target",
		"contradicts": "Source note contradicts or conflicts with target",
		"extends":     "Source note extends or elaborates on target",
		"causes":      "Source note describes a cause of the target",
		"example-of":  "Source note is an example of the concept in target",
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
	fmt.Fprintln(&b, "Run `zk schema <resource>` for detailed field information.")
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
	Long:  "Display available resources and their field definitions. Without arguments, lists all resources. With a resource name, shows detailed field schema.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := buildSchemaRegistry()
		f := getFormatter()

		if len(args) == 0 {
			// List mode: show all resources without fields.
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

		// Detail mode: show full schema for the specified resource.
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
