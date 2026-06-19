package target

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/openmcp-project/controller-utils/pkg/clusters"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	mcpv2 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
	pwv1alpha1 "github.com/openmcp-project/project-workspace-operator/api/core/v1alpha1"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	libutils "github.com/Diaphteiros/kw/pluginlib/pkg/utils"

	"github.com/Diaphteiros/kw_mcpu/pkg/config"
	"github.com/Diaphteiros/kw_mcpu/pkg/state"
)

const PromptForArg = "<prompt>"

var (
	cs                          *callState
	req                         libutils.Requirements
	onboardingCluster           *clusters.Cluster
	onboardingClusterKubeconfig []byte
)

var TargetCmd = &cobra.Command{
	Use:                "target [flags]",
	DisableAutoGenTag:  true,
	DisableFlagParsing: true,
	Args:               cobra.ArbitraryArgs,
	Short:              "Switch to a ControlPlane cluster",
	Long: `Switch to a ControlPlane cluster.

This command can be used to switch the kubeconfig to a cluster belonging to an openMCP landscape.

The following arguments specify the target cluster:
- --landscape/-l <name>: The MCP landscape to target.
- --project/-p <name>: The project (project namespace on the onboarding cluster) to target.
- --workspace/-w <name>: The workspace (workspace namespace on the onboarding cluster) to target.
- --controlplane/-c <name>: The ControlPlane cluster to target.

Targeting a landscape does not have any requirements, except from the landscape being defined in the plugin configuration.

Targeting a project requires the landscape to be either set via the corresponding argument or recoverable from the kubeswitcher state.
It results in the onboarding cluster being targeted, with the default namespace in the kubeconfig set to the project namespace.

Targeting a workspace requires the project to be either set via the corresponding argument or recoverable from the kubeswitcher state, and thus also the landscape.
It results in the onboarding cluster being targeted, with the default namespace in the kubeconfig set to the workspace namespace.

Targeting a ControlPlane cluster requires landscape, project, and workspace to be either set via the corresponding arguments or recoverable from the kubeswitcher state.

All of the '--landscape', '--project', '--workspace', and '--controlplane' flags can be specified with or without an argument. If specified without, you will be prompted to select the value interactively.
If the argument is required, but not specified at all, the command fails if the value cannot be recovered from the current kubeswitcher state.

Examples:

	# Target the onboarding cluster of the 'live' landscape.
	kw mcpu target --landscape live

	# Target the project 'my-project' on the landscape which is currently active in the kubeswitcher state (= was selected by a previous 'kw mcpu target' call).
	# Fails if the landscape cannot be recovered from the state.
	kw mcpu target --project my-project

	# Target the cluster belonging to the ControlPlane 'foo' on the 'live' landscape, in the project 'my-project' and the workspace 'my-ws'.
	kw mcpu target --landscape live --project my-project --workspace my-ws --controlplane foo

	# Target a cluster belonging to a ControlPlane. Prompts for landscape, project, workspace and ControlPlane selection.
	kw mcpu target -l -p -w -c`,
	Run: func(cmd *cobra.Command, args []string) {
		if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			if err := cmd.Help(); err != nil {
				cmd.PrintErrf("unable to print help: %v", err)
			}
			return
		}
		// parse flags
		parseArgs(cmd, args)
		// validate arguments
		validateArgs()

		// load context and config
		debug.Debug("Loading kubeswitcher context from environment")
		con, err := libcontext.NewContextFromEnv()
		if err != nil {
			libutils.Fatal(1, "error creating kubeswitcher context from environment (this is a plugin, did you run it as standalone?): %w\n", err)
		}
		debug.Debug("Kubeswitcher context loaded:\n%s", con.String())
		debug.Debug("Loading plugin configuration")
		cfg, err := config.LoadFromBytes([]byte(con.PluginConfig))
		if err != nil {
			libutils.Fatal(1, "error loading plugin configuration: %w\n", err)
		}
		debug.Debug("Plugin configuration loaded:\n%s", cfg.String())

		// load previous state, if possible
		cs = &callState{}
		debug.Debug("Loading original state, if possible")
		cs.OriginalState = &state.MCPState{}
		loaded, err := cs.OriginalState.Load(con, cfg)
		if err != nil {
			libutils.Fatal(1, "error loading plugin state: %w\n", err)
		}
		if loaded {
			debug.Debug("Loaded original state")
			// load the kubeconfig for the original state
			kcfgData, err := os.ReadFile(con.KubeconfigPath)
			if err != nil {
				libutils.Fatal(1, "error reading kubeconfig file from path '%s': %w\n", con.KubeconfigPath, err)
			}
			cs.OriginalStateKubeconfig = kcfgData
			debug.Debug("Loaded kubeconfig for original state")
		} else {
			cs.OriginalState = nil
		}
		cs.CurrentState = &state.MCPState{
			Focus: state.NewEmptyFocus(),
		}

		// print the call state for debugging purposes
		debug.Debug("Current call state:\n")
		pData, err := yaml.Marshal(cs)
		if err != nil {
			debug.Debug("Error marshaling call state data to yaml: %v", err)
		} else {
			debug.Debug("%s", string(pData))
		}

		debug.Debug("Command called with the following arguments:\n  --landscape: %s\n  --project: %s\n  --workspace: %s\n  --controlplane: %s", landscapeArg, projectArg, workspaceArg, cpArg)

		// setup requirements
		// has to happen here due to required context
		req.Register(reqLandscape, satisfyLandscapeRequirement(cfg))
		req.Register(reqOnboardingCluster, satisfyOnboardingClusterRequirement(con, cfg))
		req.Register(reqNamespaces, satisfyNamespacesRequirement(cmd))
		req.Register(reqProject, satisfyProjectRequirement(cmd))
		req.Register(reqWorkspace, satisfyWorkspaceRequirement(cmd))
		req.Register(reqControlPlane, satisfyControlPlaneRequirement(cmd))

		// we always need the onboarding cluster
		if err := req.Require(reqLandscape, reqOnboardingCluster); err != nil {
			libutils.Fatal(1, "error getting access to onboarding cluster: %w\n", err)
		}
		var kcfgData []byte
		cs.CurrentState.Focus.ToLandscape(cs.LandscapeName)
		if projectArg != "" || workspaceArg != "" || cpArg != "" {
			debug.Debug("determining project")
			if err := req.Require(reqProject); err != nil {
				libutils.Fatal(1, "error determining project: %w\n", err)
			}
			cs.CurrentState.Focus.ToProject(cs.Project.Name)
		}
		if workspaceArg != "" || cpArg != "" {
			debug.Debug("determining workspace")
			if err := req.Require(reqWorkspace); err != nil {
				libutils.Fatal(1, "error determining workspace: %w\n", err)
			}
			cs.CurrentState.Focus.ToWorkspace(cs.Workspace.Name)
		}
		if cpArg != "" {
			debug.Debug("determining ControlPlane cluster")
			if err := req.Require(reqControlPlane); err != nil {
				libutils.Fatal(1, "error determining ControlPlane cluster: %w\n", err)
			}
			cs.CurrentState.Focus.ToControlPlane(cs.ControlPlane.Namespace, cs.ControlPlane.Name)
			kcfgData, err = fetchControlPlaneKubeconfig(cmd.Context(), cfg)
			if err != nil {
				libutils.Fatal(1, "error fetching ControlPlane kubeconfig: %w\n", err)
			}
			debug.Debug("Successfully fetched kubeconfig for ControlPlane cluster")
		} else {
			debug.Debug("No ControlPlane specified, targeting onboarding cluster")
			switch cs.CurrentState.Focus.Focus() {
			case state.FocusTypeLandscape:
				kcfgData, err = withDefaultNamespace(onboardingClusterKubeconfig, "default")
			case state.FocusTypeProject:
				kcfgData, err = withDefaultNamespace(onboardingClusterKubeconfig, cs.Project.Status.Namespace)
			case state.FocusTypeWorkspace:
				kcfgData, err = withDefaultNamespace(onboardingClusterKubeconfig, cs.Workspace.Status.Namespace)
			default:
				libutils.Fatal(1, "unexpected focus type '%s'", cs.CurrentState.Focus.Focus())
			}
			if err != nil {
				libutils.Fatal(1, "error setting default namespace in kubeconfig: %w\n", err)
			}
		}

		// write state and output kubeconfig
		if cs.OriginalState != nil && cs.OriginalState.Focus.Equal(cs.CurrentState.Focus) && bytes.Equal(cs.OriginalStateKubeconfig, kcfgData) {
			debug.Debug("State has not changed, skipping writing it")
			return
		}
		if err := con.WriteKubeconfig(kcfgData, cs.CurrentState.Notification()); err != nil {
			libutils.Fatal(1, "error writing kubeconfig and/or notification: %w\n", err)
		}
		if err := con.WriteId(cs.CurrentState.Id(con.CurrentPluginName)); err != nil {
			libutils.Fatal(1, "error writing state ID: %w\n", err)
		}
		if err := con.WritePluginState(cs.CurrentState); err != nil {
			libutils.Fatal(1, "error writing plugin state: %w\n", err)
		}
	},
}

// callState is used to store information during internal calls to other plugins
type callState struct {
	LandscapeName           string                `json:"landscapeName,omitempty"`
	AccessibleNamespaces    AccessibleNamespaces  `json:"accessibleNamespaces,omitempty"`
	Project                 *pwv1alpha1.Project   `json:"project,omitempty"`
	Workspace               *pwv1alpha1.Workspace `json:"workspace,omitempty"`
	ControlPlane            *mcpv2.ControlPlane   `json:"controlPlane,omitempty"`
	CurrentState            *state.MCPState       `json:"currentState,omitempty"`  // holds the current state of the plugin
	OriginalState           *state.MCPState       `json:"originalState,omitempty"` // holds the state of the plugin when the command was called
	OriginalStateKubeconfig []byte                `json:"originalStateKubeconfig,omitempty"`
}

type AccessibleNamespaces []AccessibleNamespace

type AccessibleNamespace struct {
	Name      string `json:"name"`
	Project   string `json:"project,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

// Projects returns a mapping from project names to their corresponding namespaces.
// If directlyAccessibleOnly is true, only projects where their namespace is accessible are returned.
// Otherwise, also projects which contain an accessible Workspace are returned, but their namespaces are not known and the corresponding values in the returned map will be empty strings.
func (ans AccessibleNamespaces) Projects(directlyAccessibleOnly bool) map[string]string {
	res := map[string]string{}
	for _, an := range ans {
		if an.Project != "" {
			if an.Workspace == "" {
				res[an.Project] = an.Name
			} else if _, ok := res[an.Project]; !ok && !directlyAccessibleOnly {
				res[an.Project] = ""
			}
		}
	}
	return res
}

// Workspaces returns a mapping from workspace names to their corresponding namespaces.
// If project is not empty, only workspaces belonging to the specified project are returned.
func (ans AccessibleNamespaces) Workspaces(project string) map[string]string {
	res := map[string]string{}
	for _, an := range ans {
		if an.Workspace != "" && (project == "" || an.Project == project) {
			res[an.Workspace] = an.Name
		}
	}
	return res
}

// withDefaultNamespace takes a kubeconfig as bytes and returns the modified kubeconfig with the default namespace set to the given namespace.
// It returns a new byte slice and does not modify the input kubeconfig.
func withDefaultNamespace(original []byte, namespace string) ([]byte, error) {
	debug.Debug("Setting default namespace in kubeconfig to '%s'", namespace)

	kcfg, err := libutils.ParseKubeconfig(original)
	if err != nil {
		return nil, fmt.Errorf("error parsing kubeconfig: %w", err)
	}
	curCtx, ok := kcfg.Contexts[kcfg.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("invalid kubeconfig: current context '%s' not found", kcfg.CurrentContext)
	}
	curCtx.Namespace = namespace
	kcfgData, err := clientcmd.Write(*kcfg)
	if err != nil {
		return nil, fmt.Errorf("error marshalling kubeconfig: %w", err)
	}
	return kcfgData, nil
}

// fetchControlPlaneKubeconfig tries to fetch the kubeconfig for the selected ControlPlane cluster
// This requires project, workspace, and ControlPlane to be known, as well as at least one access configuration in the plugin config that matches the selected ControlPlane.
func fetchControlPlaneKubeconfig(ctx context.Context, cfg *config.MCPConfig) ([]byte, error) {
	debug.Debug("Fetching kubeconfig for ControlPlane '%s/%s'", cs.ControlPlane.Namespace, cs.ControlPlane.Name)
	// identify access configuration for the ControlPlane, if any
	var ac *config.AccessConfig
	for _, aCfg := range cfg.Landscapes[cs.LandscapeName].ControlPlaneAccess {
		if aCfg.Selectors.Project.Matches(cs.Project) && aCfg.Selectors.Workspace.Matches(cs.Workspace) && aCfg.Selectors.ControlPlane.Matches(cs.ControlPlane) {
			ac = &aCfg.Access
			break
		}
	}
	var secretRef *commonapi.LocalObjectReference
	if ac != nil {
		debug.Debug("Found access configuration of type '%s' for the selected ControlPlane, uses '%s'", ac.Type, ac.Name)
		key := fmt.Sprintf("%s_%s", string(ac.Type), ac.Name)
		sf, ok := cs.ControlPlane.Status.Access[key]
		if !ok {
			return nil, fmt.Errorf("plugin config specifies to use access '%s' of type '%s' for the selected ControlPlane, but the ControlPlane's status does not contain a secret reference with key '%s'", ac.Name, ac.Type, key)
		}
		secretRef = &sf
	} else {
		debug.Debug("Plugin config does not contain an access configuration which matches project '%s', workspace '%s', and ControlPlane '%s'", cs.Project.Name, cs.Workspace.Name, cs.ControlPlane.Name)
		// Fallback: If the ControlPlane has only a single access configured, use that one.
		if len(cs.ControlPlane.Status.Access) == 1 {
			debug.Debug("Using fallback: ControlPlane has only a single access configured")
			for _, ref := range cs.ControlPlane.Status.Access {
				secretRef = &ref
				break
			}
		}
	}
	if secretRef == nil {
		return nil, fmt.Errorf("unable to determine ControlPlane access to use (this happens if the ControlPlane has more than one access configured and the plugin configuration does not specify which one to use)")
	}
	s := &corev1.Secret{}
	s.Name = secretRef.Name
	s.Namespace = cs.ControlPlane.Namespace
	debug.Debug("Fetching secret '%s/%s' from onboarding cluster", s.Namespace, s.Name)
	if err := onboardingCluster.Client().Get(ctx, client.ObjectKeyFromObject(s), s); err != nil {
		return nil, fmt.Errorf("error fetching ControlPlane access secret from onboarding cluster: %w", err)
	}
	kcfgData, ok := s.Data[clustersv1alpha1.SecretKeyKubeconfig]
	if !ok {
		return nil, fmt.Errorf("secret '%s/%s' does not contain kubeconfig data under key '%s'", s.Namespace, s.Name, clustersv1alpha1.SecretKeyKubeconfig)
	}
	return kcfgData, nil
}
