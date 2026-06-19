package target

import (
	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	landscapeArg string
	projectArg   string
	workspaceArg string
	cpArg        string
)

func init() {
	req = libutils.NewRequirements()

	// This is just for generating the help message, flag parsing needs to be done manually and happens in parseArgs.
	TargetCmd.Flags().StringVarP(&landscapeArg, "landscape", "l", "", "The openMCP landscape to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.")
	TargetCmd.Flags().StringVarP(&projectArg, "project", "p", "", "The openMCP project to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.")
	TargetCmd.Flags().StringVarP(&workspaceArg, "workspace", "w", "", "The openMCP workspace to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.")
	TargetCmd.Flags().StringVarP(&cpArg, "controlplane", "c", "", "The ControlPlane cluster to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.")
}

func validateArgs() {
	// nothing to validate for now
}

// parseArgs parses the command line flags
// We cannot use the cobra-native coding here, because we want some flags to have an optional argument (determined by whether the next argument starts with a '-'), which cobra does not support.
func parseArgs(cmd *cobra.Command, args []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--landscape", "-l":
			if i+1 < len(args) && !isFlag(args[i+1]) {
				landscapeArg = args[i+1]
				i++
			} else {
				landscapeArg = PromptForArg
			}
		case "--project", "-p":
			if i+1 < len(args) && !isFlag(args[i+1]) {
				projectArg = args[i+1]
				i++
			} else {
				projectArg = PromptForArg
			}
		case "--workspace", "-w":
			if i+1 < len(args) && !isFlag(args[i+1]) {
				workspaceArg = args[i+1]
				i++
			} else {
				workspaceArg = PromptForArg
			}
		case "--controlplane", "-c":
			if i+1 < len(args) && !isFlag(args[i+1]) {
				cpArg = args[i+1]
				i++
			} else {
				cpArg = PromptForArg
			}
		default:
			if err := cmd.Usage(); err != nil {
				cmd.PrintErrf("unable to print usage info: %v", err)
			}
			libutils.Fatal(1, "unknown flag '%s'\n", arg)
		}
	}
}

func isFlag(arg string) bool {
	return len(arg) > 0 && arg[0] == '-'
}
