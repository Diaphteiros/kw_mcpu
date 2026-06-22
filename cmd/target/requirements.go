package target

import (
	"fmt"
	"os"
	"slices"
	"strings"

	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/openmcp-project/controller-utils/pkg/clusters"
	mcpv2 "github.com/openmcp-project/openmcp-operator/api/core/v2alpha1"
	mcpv2install "github.com/openmcp-project/openmcp-operator/api/install"
	pwv1alpha1 "github.com/openmcp-project/project-workspace-operator/api/core/v1alpha1"
	pwinstall "github.com/openmcp-project/project-workspace-operator/api/install"
	"github.com/spf13/cobra"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	"github.com/Diaphteiros/kw/pluginlib/pkg/selector"

	"github.com/Diaphteiros/kw_mcpu/pkg/config"
	"github.com/Diaphteiros/kw_mcpu/pkg/state"
)

const (
	reqLandscape         = "landscape"
	reqProject           = "project"
	reqWorkspace         = "workspace"
	reqControlPlane      = "controlPlane"
	reqOnboardingCluster = "onboardingCluster"
	reqNamespaces        = "namespaces"

	// hard-coding here to avoid dependency towards platform service project-workspace
	// (unfortunately, the labels are not defined in the API packages)
	ProjectLabel   = pwv1alpha1.GroupName + "/project"
	WorkspaceLabel = pwv1alpha1.GroupName + "/workspace"
)

// This file provides satisfyer methods for the requirements logic from the utils library.
// A key (the constants above) can be registered together with a function that satisfies the corresponding requirement (these are the functions below).
// When req.Require(key1, key2, ...) is called, the corresponding satisfyer functiones are called, unless they have been called before.
// It is basically a fancy way of ensuring that some code has been run exactly once before doing something.

// landscape requirement
// If satisfied, cs.LandscapeName can be expected to be a non-empty string.
func satisfyLandscapeRequirement(cfg *config.MCPConfig) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqLandscape)
		if cs.LandscapeName == "" {
			if landscapeArg == PromptForArg {
				debug.Debug("Prompting for landscape name.")
				landscapeList := sets.KeySet(cfg.Landscapes).UnsortedList()
				slices.SortFunc(landscapeList, func(a, b string) int {
					return -strings.Compare(a, b)
				})
				// select MCP landscape
				_, cs.LandscapeName, _ = selector.New[string]().
					WithPrompt("Select MCP landscape: ").
					WithFatalOnAbort("No landscape selected.").
					WithFatalOnError("error selecting landscape: %w").
					From(landscapeList, func(elem string) string { return elem }).
					Select()
				debug.Debug("Selected Landscape: %s", cs.LandscapeName)
			} else {
				cs.LandscapeName = landscapeArg
			}
		}
		if cs.LandscapeName == "" {
			debug.Debug("No landscape specified via arguments, trying to retrieve it from state.")
			if cs.OriginalState != nil && cs.OriginalState.Focus.Landscape != "" {
				cs.LandscapeName = cs.OriginalState.Focus.Landscape
			}
			if cs.LandscapeName != "" {
				debug.Debug("Identified landscape '%s' from state.", cs.LandscapeName)
			} else {
				return fmt.Errorf("unable to infer landscape name from previous command's state, specify it via '--landscape' flag")
			}
		}
		return nil
	}
}

// helper for onboarding cluster requirement
// If satisfied, the onboardingCluster variable can be expected to be ready for use.
// The onboardingClusterKubeconfig variable will also be set.
func satisfyOnboardingClusterRequirement(con *libcontext.Context, cfg *config.MCPConfig) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqOnboardingCluster)
		if err := handlePrerequisites(reqOnboardingCluster, reqLandscape); err != nil {
			return err
		}
		onboardingCluster = clusters.New("onboarding")
		if cs.OriginalState != nil && cs.OriginalState.Focus.Landscape == cs.LandscapeName && cs.OriginalState.Focus.Focus() == state.FocusTypeLandscape && len(cs.OriginalState.OnboardingClusterKubeconfig) > 0 {
			// original state is pointing to the desired cluster, we can use it
			debug.Debug("Using onboarding cluster from original state")
			if err := onboardingCluster.WithConfigPath(con.KubeconfigPath).InitializeRESTConfig(); err != nil {
				return fmt.Errorf("error initializing REST config for onboarding cluster from original state kubeconfig: %w", err)
			}
			onboardingClusterKubeconfig = cs.OriginalState.OnboardingClusterKubeconfig
		} else {
			// we need to load the kubeconfig from the landscape config
			debug.Debug("Loading onboarding cluster kubeconfig from landscape config")
			lsCfg, ok := cfg.Landscapes[cs.LandscapeName]
			if !ok {
				return fmt.Errorf("landscape '%s' not found in config", cs.LandscapeName)
			}
			if len(lsCfg.OnboardingKubeconfig.Inline) > 0 {
				debug.Debug("Using inline kubeconfig from config")
				restCfg, err := clientcmd.RESTConfigFromKubeConfig(lsCfg.OnboardingKubeconfig.Inline)
				if err != nil {
					return fmt.Errorf("error creating REST config from inline kubeconfig: %w", err)
				}
				onboardingCluster.WithRESTConfig(restCfg)
				onboardingClusterKubeconfig = lsCfg.OnboardingKubeconfig.Inline
			} else if lsCfg.OnboardingKubeconfig.Path != "" {
				debug.Debug("Using kubeconfig from path '%s'", lsCfg.OnboardingKubeconfig.Path)
				if err := onboardingCluster.WithConfigPath(lsCfg.OnboardingKubeconfig.Path).InitializeRESTConfig(); err != nil {
					return fmt.Errorf("error initializing REST config for onboarding cluster from kubeconfig path: %w", err)
				}
				data, err := os.ReadFile(lsCfg.OnboardingKubeconfig.Path)
				if err != nil {
					return fmt.Errorf("error reading kubeconfig from path '%s': %w", lsCfg.OnboardingKubeconfig.Path, err)
				}
				onboardingClusterKubeconfig = data
			}
		}
		sc := runtime.NewScheme()
		pwinstall.InstallOperatorAPIsOnboarding(sc)
		mcpv2install.InstallOperatorAPIsOnboarding(sc)
		if err := onboardingCluster.InitializeClient(sc); err != nil {
			return fmt.Errorf("error initializing client for onboarding cluster: %w", err)
		}
		return nil
	}
}

// namespaces requirement
// If satisfied, cs.AccessibleNamespaces can be expected to be set.
func satisfyNamespacesRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqNamespaces)
		if err := handlePrerequisites(reqNamespaces, reqOnboardingCluster); err != nil {
			return err
		}
		debug.Debug("Listing accessible namespaces")
		// end-users can only access namespaces belonging to their projects and workspaces
		ssrr := &authzv1.SelfSubjectRulesReview{
			Spec: authzv1.SelfSubjectRulesReviewSpec{
				Namespace: "*",
			},
		}
		ssrr.SetName("onboarding") // we can set any name here, as it is not used by the API
		if err := onboardingCluster.Client().Create(cmd.Context(), ssrr); err != nil {
			return fmt.Errorf("error creating SelfSubjectRulesReview in onboarding cluster: %w", err)
		}
		nsNames := sets.New[string]()
		for _, rule := range ssrr.Status.ResourceRules {
			// search for namespaces where the user has access
			if slices.Contains(rule.APIGroups, corev1.GroupName) && slices.Contains(rule.Resources, "namespaces") && slices.Contains(rule.Verbs, "get") {
				for _, rn := range rule.ResourceNames {
					nsNames.Insert(rn)
				}
			}
		}

		// fetch all namespaces for more information
		for nsName := range nsNames {
			ns := &corev1.Namespace{}
			if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKey{Name: nsName}, ns); err != nil {
				return fmt.Errorf("error fetching namespace '%s': %w", nsName, err)
			}
			an := AccessibleNamespace{
				Name: ns.Name,
			}
			if project, ok := ns.Labels[ProjectLabel]; ok {
				an.Project = project
			}
			if workspace, ok := ns.Labels[WorkspaceLabel]; ok {
				an.Workspace = workspace
			}
			cs.AccessibleNamespaces = append(cs.AccessibleNamespaces, an)
		}

		// log accessible namespaces for debugging purposes
		anYaml, err := yaml.Marshal(cs.AccessibleNamespaces)
		if err != nil {
			debug.Debug("unable to marshal accessible namespaces to yaml: %v", err)
		} else {
			debug.Debug("Accessible namespaces:\n%s", string(anYaml))
		}

		return nil
	}
}

// project requirement
// If satisfied, cs.Project can be expected to be set.
// Note that some projects might not be accessible directly, resulting in their resource being a mock with only the name set.
func satisfyProjectRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqProject)
		projectName := ""
		if projectArg != PromptForArg {
			// option 1: project name is provided via argument or can be recovered from state
			if projectArg != "" {
				projectName = projectArg
			} else if landscapeArg == "" {
				// try to derive project name from state, but only if the landscape was not explicitly specified
				debug.Debug("No project name specified via arguments, trying to retrieve it from state.")
				if cs.OriginalState != nil && cs.OriginalState.Focus != nil && cs.OriginalState.Focus.Project != "" {
					projectName = cs.OriginalState.Focus.Project
					debug.Debug("Identified project '%s' from state.", projectName)
				}
			}
			if projectName != "" {
				if workspaceArg == "" && cpArg == "" {
					// The project is targeted directly, so we can fetch it (user needs permission to do so anyway to target it)
					debug.Debug("Fetching project '%s'", projectName)
					if err := handlePrerequisites(reqProject, reqOnboardingCluster); err != nil {
						return err
					}
					project := &pwv1alpha1.Project{}
					project.Name = projectName
					if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(project), project); err != nil {
						return fmt.Errorf("unable to get project '%s' on onboarding cluster: %w", project.Name, err)
					}
					cs.Project = project
				} else {
					// The project is not the primary target of the command, so we don't need anything except for the project name, which we have - let's just mock the project resource
					debug.Debug("Mocking project, as just its name is required")
					cs.Project = &pwv1alpha1.Project{}
					cs.Project.Name = projectName
				}
			}
		} else {
			// option 2: prompt requested for project name, this will fetch the project as a side-effect
			if err := handlePrerequisites(reqProject, reqOnboardingCluster, reqNamespaces); err != nil {
				return err
			}
			directlyAccessibleProjectsOnly := false
			logMod := ""
			if workspaceArg == "" && cpArg == "" {
				// This command seems to target a project directly (instead of a workspace or controlplane),
				// so let's only show projects that are actually accessible by the user.
				directlyAccessibleProjectsOnly = true
				logMod = " (directly accessible only)"
			}
			debug.Debug("Fetching accessible projects%s", logMod)
			projects := []*pwv1alpha1.Project{}
			for prName, prNamespace := range cs.AccessibleNamespaces.Projects(directlyAccessibleProjectsOnly) {
				p := &pwv1alpha1.Project{}
				p.Name = prName
				p.Status.Namespace = prNamespace
				// try to fetch accessible projects
				if prNamespace != "" {
					if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(p), p); err != nil {
						debug.Debug("Unable to fetch project '%s/%s': %v", prNamespace, prName, err)
					}
				}
				projects = append(projects, p)
			}

			debug.Debug("Prompting for project name.")
			// select project
			_, project, _ := selector.New[*pwv1alpha1.Project]().
				WithPrompt("Select project: ").
				WithFatalOnAbort("No project selected.").
				WithFatalOnError("error selecting project: %w").
				WithPreview(projectSelectorPreview).
				WithSortByKey(selector.Invert).
				From(projects, func(elem *pwv1alpha1.Project) string { return elem.Name }).
				Select()
			cs.Project = project
			debug.Debug("Selected Project: %s", cs.Project.Name)
		}
		if cs.Project == nil {
			return fmt.Errorf("unable to identify project, specify its name via the '--project' flag")
		}

		nsMod := "<unknown>"
		if cs.Project.Status.Namespace != "" {
			nsMod = cs.Project.Status.Namespace
		}
		debug.Debug("Project: %s (namespace: %s)", cs.Project.Name, nsMod)
		return nil
	}
}

// workspace requirement
// If satisfied, cs.Workspace can be expected to be set.
func satisfyWorkspaceRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqWorkspace)
		if err := handlePrerequisites(reqWorkspace, reqProject, reqNamespaces); err != nil {
			return err
		}
		wsName := ""
		if workspaceArg != PromptForArg {
			// option 1: workspace name is provided via argument or can be recovered from state
			if workspaceArg != "" {
				wsName = workspaceArg
			} else if landscapeArg == "" && projectArg == "" {
				// try to derive workspace name from state, but only if the landscape and project were not explicitly specified
				debug.Debug("No workspace name specified via arguments, trying to retrieve it from state.")
				if cs.OriginalState != nil && cs.OriginalState.Focus != nil && cs.OriginalState.Focus.Workspace != "" {
					wsName = cs.OriginalState.Focus.Workspace
					debug.Debug("Identified workspace '%s' from state.", wsName)
				}
			}
			if wsName != "" {
				debug.Debug("Searching for accessible namespace with project '%s' and workspace '%s' to identify workspace namespace", cs.Project.Name, wsName)
				workspace := &pwv1alpha1.Workspace{}
				workspace.Name = wsName
				for _, an := range cs.AccessibleNamespaces {
					if an.Workspace == wsName && an.Project == cs.Project.Name {
						workspace.Status.Namespace = an.Name
						debug.Debug("Identified workspace namespace '%s' via permissions", workspace.Status.Namespace)
						break
					}
				}
				if workspace.Status.Namespace == "" {
					return fmt.Errorf("unable to identify namespace for workspace '%s' in project '%s'", wsName, cs.Project.Name)
				}
				cs.Workspace = workspace
			}
		} else {
			// option 2: prompt requested for workspace name, this will fetch the workspace as a side-effect
			if err := handlePrerequisites(reqWorkspace, reqOnboardingCluster); err != nil {
				return err
			}
			debug.Debug("Fetching accessible workspaces")
			workspaces := []*pwv1alpha1.Workspace{}
			for wsName, wsNamespace := range cs.AccessibleNamespaces.Workspaces(cs.Project.Name) {
				w := &pwv1alpha1.Workspace{}
				w.Name = wsName
				if cs.Project.Status.Namespace != "" {
					w.Namespace = cs.Project.Status.Namespace
					// try to fetch workspaces for further information
					if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(w), w); err != nil {
						debug.Debug("Unable to fetch workspace '%s/%s': %v", wsNamespace, wsName, err)
					}
				}
				if w.Status.Namespace == "" {
					w.Status.Namespace = wsNamespace
				}
				workspaces = append(workspaces, w)
			}

			debug.Debug("Prompting for workspace name.")
			// select workspace
			_, workspace, _ := selector.New[*pwv1alpha1.Workspace]().
				WithPrompt("Select workspace: ").
				WithFatalOnAbort("No workspace selected.").
				WithFatalOnError("error selecting workspace: %w").
				WithPreview(workspaceSelectorPreview).
				WithSortByKey(selector.Invert).
				From(workspaces, func(elem *pwv1alpha1.Workspace) string { return elem.Name }).
				Select()
			cs.Workspace = workspace
			debug.Debug("Selected Workspace: %s", cs.Workspace.Name)
		}
		if cs.Workspace == nil {
			return fmt.Errorf("unable to identify workspace, specify its name via the '--workspace' flag")
		} else if cs.Workspace.Status.Namespace == "" {
			return fmt.Errorf("workspace '%s' does not have a namespace assigned", cs.Workspace.Name)
		}

		nsMod := "<unknown>"
		if cs.Workspace.Status.Namespace != "" {
			nsMod = cs.Workspace.Status.Namespace
		}
		debug.Debug("Workspace: %s (namespace: %s)", cs.Workspace.Name, nsMod)
		return nil
	}
}

// controlPlane requirement
// If satisfied, cs.ControlPlane can be expected to be set.
func satisfyControlPlaneRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqControlPlane)
		if err := handlePrerequisites(reqControlPlane, reqOnboardingCluster, reqWorkspace); err != nil {
			return err
		}
		if cpArg != PromptForArg {
			// option 1: controlplane name is provided via argument
			// => fetch corresponding workspace
			if cpArg != "" {
				cp := &mcpv2.ControlPlane{}
				cp.Name = cpArg
				cp.Namespace = cs.Workspace.Status.Namespace
				debug.Debug("Fetching ControlPlane '%s/%s'", cp.Namespace, cp.Name)
				if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(cp), cp); err != nil {
					return fmt.Errorf("unable to get ControlPlane '%s/%s' on onboarding cluster: %w", cp.Namespace, cp.Name, err)
				}
				cs.ControlPlane = cp
			}
			// It doesn't really make sense to recover the ControlPlane name from state, because if the plugin is used, the user probably wants to target something else than what is currently targeted.
		} else {
			debug.Debug("Listing ControlPlanes in namespace '%s'", cs.Workspace.Status.Namespace)
			cpList := &mcpv2.ControlPlaneList{}
			if err := onboardingCluster.Client().List(cmd.Context(), cpList, client.InNamespace(cs.Workspace.Status.Namespace)); err != nil {
				return fmt.Errorf("unable to list ControlPlanes in namespace '%s' on onboarding cluster: %w", cs.Workspace.Status.Namespace, err)
			}
			debug.Debug("Prompting for ControlPlane name.")
			// select ControlPlane
			_, cp, _ := selector.New[mcpv2.ControlPlane]().
				WithPrompt("Select ControlPlane: ").
				WithFatalOnAbort("No ControlPlane selected.").
				WithFatalOnError("error selecting ControlPlane: %w").
				WithPreview(cpSelectorPreview).
				WithSortByKey(selector.Invert).
				From(cpList.Items, func(elem mcpv2.ControlPlane) string { return elem.Name }).
				Select()
			cs.ControlPlane = &cp
			debug.Debug("Selected ControlPlane: %s", cs.ControlPlane.Name)
		}

		if cs.ControlPlane == nil {
			return fmt.Errorf("unable to identify ControlPlane, specify its name via the '--controlplane' flag")
		}
		return nil
	}
}

func handlePrerequisites(key string, prerequisites ...string) error {
	debug.Debug("Satisfying prerequisites for requirement '%s': %v", key, prerequisites)
	if err := req.Require(prerequisites...); err != nil {
		return fmt.Errorf("error satisfying prerequisites for requirement '%s': %w", key, err)
	}
	return nil
}
