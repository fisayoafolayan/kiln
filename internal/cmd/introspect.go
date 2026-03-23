package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func introspectCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "introspect",
		Short: "Print the parsed schema IR (useful for debugging)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, schema, err := parseSchema()
			if err != nil {
				return err
			}

			switch format {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(schema)
			default:
				fmt.Printf("Driver: %s\n", schema.Driver)
				fmt.Printf("Tables: %d\n\n", len(schema.Tables))
				for _, t := range schema.Tables {
					fmt.Printf("  %s (%d columns)\n", t.Name, len(t.Columns))
					for _, c := range t.Columns {
						pk := ""
						if c.IsPrimaryKey {
							pk = " [PK]"
						}
						nullable := ""
						if c.Nullable {
							nullable = "?"
						}
						fmt.Printf("    %-24s %s%s%s\n", c.Name, c.GoType.String(), nullable, pk)
					}
					if len(t.ForeignKeys) > 0 {
						fmt.Printf("  Foreign keys:\n")
						for _, fk := range t.ForeignKeys {
							fmt.Printf("    %s.%s → %s.%s\n",
								fk.SourceTable.Name, fk.SourceColumn.Name,
								fk.TargetTable.Name, fk.TargetColumn.Name,
							)
						}
					}
					fmt.Println()
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json")
	return cmd
}
