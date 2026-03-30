package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillDoc string

// skillCmd returns the "skill" command, which prints the LLM-oriented command reference.
func skillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Print the LLM skill reference for this CLI",
		Run: func(*cobra.Command, []string) {
			fmt.Print(skillDoc)
		},
	}
}
