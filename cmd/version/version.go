package version

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"sigs.k8s.io/yaml"

	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"

	"github.com/Diaphteiros/kw_mcpu/internal/version"
)

// variables for holding the flags
var (
	output libutils.OutputFormat
)

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Args:    cobra.NoArgs,
	Short:   "Print the version",
	Long:    `Output the version of the CLI.`,
	Example: `  > kw version
  v1.2.3

  > kw version -o json
  {"major":"v1","minor":"2","gitVersion":"v1.2.3","gitCommit":"76c01d5337fc9de6e053b4e5bafd5239c8b7a973","gitTreeState":"dirty","buildDate":"2024-04-26T11:29:39+02:00","goVersion":"go1.22.2","compiler":"gc","platform":"darwin/arm64"}

  > kw version -o yaml
  buildDate: "2024-04-26T11:29:39+02:00"
  compiler: gc
  gitCommit: 76c01d5337fc9de6e053b4e5bafd5239c8b7a973
  gitTreeState: dirty
  gitVersion: v1.2.3
  goVersion: go1.22.2
  major: v1
  minor: "2"
  platform: darwin/arm64`,
	Run: func(cmd *cobra.Command, args []string) {
		ver := version.Get()
		switch output {
		case libutils.OUTPUT_TEXT:
			cmd.Print(ver.String())
		case libutils.OUTPUT_JSON:
			data, err := json.Marshal(ver)
			if err != nil {
				libutils.Fatal(1, "error converting version to json: %w\n", err)
			}
			cmd.Println(string(data))
		case libutils.OUTPUT_YAML:
			data, err := yaml.Marshal(ver)
			if err != nil {
				libutils.Fatal(1, "error converting version to yaml: %w\n", err)
			}
			cmd.Print(string(data))
		default:
			libutils.Fatal(1, "unknown output format '%s'\n", string(output))
		}
	},
}

func init() {
	libutils.AddOutputFlag(VersionCmd.Flags(), &output, libutils.OUTPUT_TEXT)
}
