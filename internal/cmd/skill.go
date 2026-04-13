package cmd

import (
	"fmt"

	dexcli "github.com/somnia-chain/somnia-dex-cli"
	"github.com/spf13/cobra"
)

// skillCmd returns the "skill" command, which prints the LLM-oriented command reference.
func skillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Print the LLM skill reference for this CLI",
		Run: func(*cobra.Command, []string) {
			fmt.Print(dexcli.SkillDoc)
		},
	}
}
