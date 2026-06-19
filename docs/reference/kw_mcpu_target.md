## kw_mcpu target

Switch to a ControlPlane cluster

### Synopsis

Switch to a ControlPlane cluster.

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
	kw mcpu target -l -p -w -c

```
kw_mcpu target [flags]
```

### Options

```
  -c, --controlplane string   The ControlPlane cluster to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.
  -h, --help                  help for target
  -l, --landscape string      The openMCP landscape to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.
  -p, --project string        The openMCP project to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.
  -w, --workspace string      The openMCP workspace to target. Will be prompted for if specified without an argument. Might be recovered from state, if not specified.
```

### SEE ALSO

* [kw_mcpu](kw_mcpu.md)	 - Interact with an openMCP landscape

