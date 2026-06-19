package config

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	openmcpinstall "github.com/openmcp-project/openmcp-operator/api/install"

	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
)

var Scheme *runtime.Scheme

func init() {
	Scheme = runtime.NewScheme()
	openmcpinstall.InstallOperatorAPIsOnboarding(Scheme)
}

type MCPConfig struct {
	// Landscapes describes the MCP landscapes and how to reach their respective clusters.
	Landscapes map[string]*MCPLandscape `json:"landscapes"`
}

type MCPLandscape struct {
	// OnboardingKubeconfig contains the kubeconfig for the landscape's onboarding cluster.
	OnboardingKubeconfig KubeconfigClusterAccess `json:"onboardingKubeconfig"`
	// ControlPlaneAccess describes how the different ControlPlanes in the landscape can be accessed.
	ControlPlaneAccess []ControlPlaneAccessConfig `json:"controlPlaneAccess,omitempty"`
}

type KubeconfigClusterAccess struct {
	// Path to the kubeconfig file.
	// Mutually exclusive with the inline option.
	Path string `json:"path,omitempty"`
	// Inline kubeconfig.
	// Mutually exclusive with the path option.
	Inline []byte `json:"inline,omitempty"`
}

type ControlPlaneAccessConfig struct {
	// Selectors describes which ControlPlanes this access configuration applies to.
	Selectors SelectorConfig `json:"selectors"`
	// Access describes how to access the ControlPlane.
	Access AccessConfig `json:"access"`
}

type SelectorConfig struct {
	// Project specifies the selector for projects.
	// Matches all projects if nil or empty.
	Project *ObjectIdentitySelector `json:"project,omitempty"`
	// Workspace specifies the selector for workspaces.
	// Matches all workspaces if nil or empty.
	Workspace *ObjectIdentitySelector `json:"workspace,omitempty"`
	// ControlPlane specifies the selector for ControlPlanes.
	// Matches all ControlPlanes if nil or empty.
	ControlPlane *ObjectIdentitySelector `json:"controlPlane,omitempty"`
}

type AccessConfig struct {
	// Type is the type of the access.
	// Must be one of 'oidc' or 'token'.
	Type AccessType `json:"type"`
	// Name is the name of the access.
	Name string `json:"name"`
}

type AccessType string

const (
	AccessTypeOIDC  AccessType = "oidc"
	AccessTypeToken AccessType = "token"
)

func (c *MCPConfig) String() string {
	if c == nil {
		return ""
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("error marshaling config: %v", err)
	}
	return string(data)
}

func (c *MCPConfig) Default() {
	// Nothing to default at the moment.
}

func (c *MCPConfig) Validate() error {
	errs := field.ErrorList{}

	fldPath := field.NewPath("landscapes")
	for landscape, lsCfg := range c.Landscapes {
		if landscape == "" {
			errs = append(errs, field.Invalid(fldPath, landscape, "landscape name must not be empty"))
		}
		lPath := fldPath.Key(landscape)

		if (lsCfg.OnboardingKubeconfig.Path == "") == (len(lsCfg.OnboardingKubeconfig.Inline) == 0) {
			errs = append(errs, field.Invalid(lPath.Child("onboardingKubeconfig"), lsCfg.OnboardingKubeconfig, "exactly one of path and inline must be set"))
		}

		for i, cpAccess := range lsCfg.ControlPlaneAccess {
			cpPath := lPath.Child("controlPlaneAccess").Index(i)
			if cpAccess.Access.Type != AccessTypeOIDC && cpAccess.Access.Type != AccessTypeToken {
				errs = append(errs, field.NotSupported(cpPath.Child("access", "type"), string(cpAccess.Access.Type), []string{string(AccessTypeOIDC), string(AccessTypeToken)}))
			}
			if cpAccess.Access.Name == "" {
				errs = append(errs, field.Required(cpPath.Child("access", "name"), "access name must be set"))
			}
		}
	}

	return errs.ToAggregate()
}

func LoadFromBytes(data []byte) (*MCPConfig, error) {
	cfg := &MCPConfig{}
	if len(data) > 0 {
		err := yaml.Unmarshal(data, cfg)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling kw_mcpu config: %w", err)
		}
	} else {
		debug.Debug("No kw_mcpu config provided. MCP landscape configuration is required to use the plugin!")
	}
	cfg.Default()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("error validating kw_mcpu config: %w", err)
	}
	return cfg, nil
}
