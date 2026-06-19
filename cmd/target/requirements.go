package target

import (
	"fmt"
	"os"
	"slices"
	"strings"

	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// project requirement
// If satisfied, cs.Project can be expected to be set.
func satisfyProjectRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqProject)
		if err := handlePrerequisites(reqProject, reqOnboardingCluster); err != nil {
			return err
		}
		projectName := ""
		if projectArg != PromptForArg {
			// option 1: project name is provided via argument or can be recovered from state
			// => fetch corresponding project
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
				debug.Debug("Fetching project '%s'", projectName)
				project := &pwv1alpha1.Project{}
				project.Name = projectName
				if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(project), project); err != nil {
					return fmt.Errorf("unable to get project '%s' on onboarding cluster: %w", project.Name, err)
				}
				cs.Project = project
			}
		} else {
			// option 2: prompt requested for project name, this will fetch the project as a side-effect
			debug.Debug("Listing projects")
			// end-users can only access their own projects and not list all projects, so we need to do this via a SelfSubjectRulesReview to find out which projects the user has access to
			ssrr := &authzv1.SelfSubjectRulesReview{
				Spec: authzv1.SelfSubjectRulesReviewSpec{
					Namespace: "*",
				},
			}
			ssrr.SetName("onboarding") // we can set any name here, as it is not used by the API
			if err := onboardingCluster.Client().Create(cmd.Context(), ssrr); err != nil {
				return fmt.Errorf("error creating SelfSubjectRulesReview in onboarding cluster: %w", err)
			}
			projectNames := sets.New[string]()
			for _, rule := range ssrr.Status.ResourceRules {
				// search for projects where the user has access
				if slices.Contains(rule.APIGroups, pwv1alpha1.GroupName) && slices.Contains(rule.Resources, "projects") && slices.Contains(rule.Verbs, "get") {
					for _, rn := range rule.ResourceNames {
						projectNames.Insert(rn)
					}
				}
			}

			// fetch each project to get more information
			projects := make([]*pwv1alpha1.Project, 0, projectNames.Len())
			for projectName := range projectNames {
				cur := &pwv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: projectName,
					},
				}
				if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(cur), cur); err != nil {
					return fmt.Errorf("error fetching project '%s': %w", cur.Name, err)
				}
				projects = append(projects, cur)
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
		} else if cs.Project.Status.Namespace == "" {
			return fmt.Errorf("project '%s' does not have a namespace assigned", cs.Project.Name)
		}

		return nil
	}
}

// workspace requirement
// If satisfied, cs.Workspace can be expected to be set.
func satisfyWorkspaceRequirement(cmd *cobra.Command) func() error {
	return func() error {
		debug.Debug("Satisfying requirement '%s'", reqWorkspace)
		if err := handlePrerequisites(reqWorkspace, reqOnboardingCluster, reqProject); err != nil {
			return err
		}
		wsName := ""
		if workspaceArg != PromptForArg {
			// option 1: workspace name is provided via argument or can be recovered from state
			// => fetch corresponding workspace
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
				debug.Debug("Fetching workspace '%s/%s'", cs.Project.Status.Namespace, wsName)
				workspace := &pwv1alpha1.Workspace{}
				workspace.Name = wsName
				workspace.Namespace = cs.Project.Status.Namespace
				if err := onboardingCluster.Client().Get(cmd.Context(), client.ObjectKeyFromObject(workspace), workspace); err != nil {
					return fmt.Errorf("unable to get workspace '%s' on onboarding cluster: %w", workspace.Name, err)
				}
				cs.Workspace = workspace
			}
		} else {
			// option 2: prompt requested for workspace name, this will fetch the workspace as a side-effect
			debug.Debug("Listing workspaces in namespace '%s'", cs.Project.Status.Namespace)
			wsList := &pwv1alpha1.WorkspaceList{}
			if err := onboardingCluster.Client().List(cmd.Context(), wsList, client.InNamespace(cs.Project.Status.Namespace)); err != nil {
				return fmt.Errorf("unable to list workspaces in namespace '%s' on onboarding cluster: %w", cs.Project.Status.Namespace, err)
			}

			debug.Debug("Prompting for workspace name.")
			// select workspace
			_, workspace, _ := selector.New[pwv1alpha1.Workspace]().
				WithPrompt("Select workspace: ").
				WithFatalOnAbort("No workspace selected.").
				WithFatalOnError("error selecting workspace: %w").
				WithPreview(workspaceSelectorPreview).
				WithSortByKey(selector.Invert).
				From(wsList.Items, func(elem pwv1alpha1.Workspace) string { return elem.Name }).
				Select()
			cs.Workspace = &workspace
			debug.Debug("Selected Workspace: %s", cs.Workspace.Name)
		}
		if cs.Workspace == nil {
			return fmt.Errorf("unable to identify workspace, specify its name via the '--workspace' flag")
		} else if cs.Workspace.Status.Namespace == "" {
			return fmt.Errorf("workspace '%s' does not have a namespace assigned", cs.Workspace.Name)
		}

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
