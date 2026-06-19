package state

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
)

type FocusType string

const (
	FocusTypeLandscape FocusType = "landscape"
	FocusTypeProject   FocusType = "project"
	FocusTypeWorkspace FocusType = "workspace"
	FocusTypeCP        FocusType = "cp"
	FocusTypeUnknown   FocusType = "unknown"
)

func (ft FocusType) Short() string {
	switch ft {
	case FocusTypeLandscape:
		return "ls"
	case FocusTypeProject:
		return "pr"
	case FocusTypeWorkspace:
		return "ws"
	default:
		return string(ft)
	}
}

type Focus struct {
	Landscape    string                     `json:"landscape"`
	Project      string                     `json:"project,omitempty"`
	Workspace    string                     `json:"workspace,omitempty"`
	ControlPlane *commonapi.ObjectReference `json:"controlPlane,omitempty"`
}

func NewEmptyFocus() *Focus {
	return &Focus{}
}

// Focus returns the type of the current focus.
// This is computed based on the fields that are set (= not an empty string).
// - Landscape + nothing else: landscape
// - Landscape + Project: project
// - Landscape + Project + Workspace: workspace
// - Landscape + Project + Workspace + ControlPlane: cp
// - Otherwise: unknown
func (f *Focus) Focus() FocusType {
	if f == nil || f.Landscape == "" {
		return FocusTypeUnknown
	}
	if f.Project == "" && f.Workspace == "" && f.ControlPlane == nil {
		return FocusTypeLandscape
	}
	if f.Project != "" && f.Workspace == "" && f.ControlPlane == nil {
		return FocusTypeProject
	}
	if f.Project != "" && f.Workspace != "" && f.ControlPlane == nil {
		return FocusTypeWorkspace
	}
	if f.Project != "" && f.Workspace != "" && f.ControlPlane != nil {
		return FocusTypeCP
	}
	return FocusTypeUnknown
}

func (f *Focus) Notification() string {
	switch f.Focus() {
	case FocusTypeLandscape:
		return fmt.Sprintf("Switched to landscape '%s'.", f.Landscape)
	case FocusTypeProject:
		return fmt.Sprintf("Switched to project '%s' in '%s' landscape.", f.Project, f.Landscape)
	case FocusTypeWorkspace:
		return fmt.Sprintf("Switched to workspace '%s' in project '%s' in '%s' landscape.", f.Workspace, f.Project, f.Landscape)
	case FocusTypeCP:
		sb := strings.Builder{}
		sb.WriteString("Switched to ControlPlane '")
		sb.WriteString(f.ControlPlane.Name)
		if f.Workspace != "" {
			sb.WriteString("' in workspace '")
			sb.WriteString(f.Workspace)
		}
		sb.WriteString("' in project '")
		sb.WriteString(f.Project)
		sb.WriteString("' in '")
		sb.WriteString(f.Landscape)
		sb.WriteString("' landscape.")
		return sb.String()
	}
	return "Switched to unknown MCP focus. This should not happen."
}

func (f *Focus) Id(pluginName string) string {
	prMod := ""
	wsMod := ""
	if f.Project != "" {
		prMod = "/" + f.Project
		if f.Workspace != "" {
			wsMod = "/" + f.Workspace
		}
	}
	cMod := ""
	if f.ControlPlane != nil {
		cMod = "/" + f.ControlPlane.Name
	}
	return fmt.Sprintf("%s:%s|%s%s%s%s", pluginName, f.Focus().Short(), f.Landscape, prMod, wsMod, cMod)
}

func (f *Focus) BackToLandscape() *Focus {
	fc := f.Focus()
	if fc != FocusTypeProject && fc != FocusTypeWorkspace && fc != FocusTypeCP {
		return f
	}
	f.Project = ""
	f.Workspace = ""
	f.ControlPlane = nil
	return f
}

func (f *Focus) BackToProject() *Focus {
	fc := f.Focus()
	if fc != FocusTypeWorkspace && fc != FocusTypeCP {
		return f
	}
	f.Workspace = ""
	f.ControlPlane = nil
	return f
}

func (f *Focus) BackToWorkspaceOrProject() *Focus {
	fc := f.Focus()
	if fc != FocusTypeCP {
		return f
	}
	f.ControlPlane = nil
	return f
}

func (f *Focus) ToLandscape(landscape string) *Focus {
	f.Landscape = landscape
	f.Project = ""
	f.Workspace = ""
	f.ControlPlane = nil
	return f
}

func (f *Focus) ToProject(project string) *Focus {
	f.Project = project
	f.Workspace = ""
	f.ControlPlane = nil
	return f
}

func (f *Focus) ToWorkspace(workspace string) *Focus {
	f.Workspace = workspace
	f.ControlPlane = nil
	return f
}

func (f *Focus) ToControlPlane(cpNamespace, cpName string) *Focus {
	f.ControlPlane = &commonapi.ObjectReference{
		Namespace: cpNamespace,
		Name:      cpName,
	}
	return f
}

// Json returns a JSON representation of the focus.
// Panics on error.
// Returns null if the focus is nil.
func (f *Focus) Json() string {
	if f == nil {
		return "null"
	}
	data, err := json.Marshal(f)
	if err != nil {
		panic(fmt.Errorf("error marshaling focus to json: %w", err))
	}
	return string(data)
}

// Yaml returns a YAML representation of the focus.
// Panics on error.
// Returns an empty string if the focus is nil.
func (f *Focus) String() string {
	if f == nil {
		return ""
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		panic(fmt.Errorf("error marshaling focus to yaml: %w", err))
	}
	return string(data)
}

// ClusterIdentity returns the identity of the currently focused cluster.
// For Landscape focus, this simply returns 'onboarding'.
// For ControlPlane focus, this returns '<namespace>/<name>' of the ControlPlane.
// For other focus types, this returns an empty string.
func (f *Focus) ClusterIdentity() string {
	switch f.Focus() {
	case FocusTypeLandscape:
		return "onboarding"
	case FocusTypeCP:
		if f.ControlPlane == nil {
			return ""
		}
		return fmt.Sprintf("%s/%s", f.ControlPlane.Namespace, f.ControlPlane.Name)
	}
	return ""
}

func (f *Focus) DeepCopy() *Focus {
	if f == nil {
		return nil
	}
	return &Focus{
		Landscape:    f.Landscape,
		Project:      f.Project,
		Workspace:    f.Workspace,
		ControlPlane: f.ControlPlane.DeepCopy(),
	}
}

func (f *Focus) Equal(other *Focus) bool {
	if f == nil && other == nil {
		return true
	}
	if f == nil || other == nil {
		return false
	}
	if f.Landscape != other.Landscape || f.Project != other.Project || f.Workspace != other.Workspace {
		return false
	}
	if (f.ControlPlane == nil) != (other.ControlPlane == nil) {
		return false
	}
	if f.ControlPlane != nil && other.ControlPlane != nil {
		if f.ControlPlane.Namespace != other.ControlPlane.Namespace || f.ControlPlane.Name != other.ControlPlane.Name {
			return false
		}
	}
	return true
}
