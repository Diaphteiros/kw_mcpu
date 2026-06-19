package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/Diaphteiros/kw_mcpu/cmd/target"
	"github.com/Diaphteiros/kw_mcpu/cmd/version"
)

var RootCmd = &cobra.Command{
	Use:               "kw_mcpu <command>",
	DisableAutoGenTag: true,
	Args:              cobra.RangeArgs(0, 1),
	Short:             "Interact with an openMCP landscape",
	Long: `Interact with an openMCP landscape.

Checkout the subcommands for more details.`,
}

func init() {
	RootCmd.SetOut(os.Stdout)
	RootCmd.SetErr(os.Stderr)
	RootCmd.SetIn(os.Stdin)

	RootCmd.AddCommand(target.TargetCmd)
	RootCmd.AddCommand(version.VersionCmd)
}
