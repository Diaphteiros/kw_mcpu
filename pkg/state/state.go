package state

import (
	"bytes"
	"fmt"

	libcontext "github.com/Diaphteiros/kw/pluginlib/pkg/context"
	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	libstate "github.com/Diaphteiros/kw/pluginlib/pkg/state"
	"sigs.k8s.io/yaml"

	"github.com/Diaphteiros/kw_mcpu/pkg/config"
)

type MCPState struct {
	Focus                       *Focus `json:"focus"`
	OnboardingClusterKubeconfig []byte `json:"onboardingClusterKubeconfig,omitempty"` // holds the kubeconfig of the onboarding cluster, if it has already been fetched
}

func (s *MCPState) copyFrom(other *MCPState) {
	s.Focus = other.Focus.DeepCopy()
	s.OnboardingClusterKubeconfig = bytes.Clone(other.OnboardingClusterKubeconfig)
}

func (s *MCPState) DeepCopy() *MCPState {
	if s == nil {
		return nil
	}
	res := &MCPState{}
	res.copyFrom(s)
	return res
}

// String returns a YAML representation of the state.
func (s *MCPState) YAML() ([]byte, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("error marshaling MCP state to yaml: %w", err)
	}
	return data, nil
}

func (s *MCPState) Id(pluginName string) string {
	return s.Focus.Id(pluginName)
}

func (s *MCPState) Notification() string {
	return s.Focus.Notification()
}

// Load fills the receiver state object with the data from the kubeswitcher state.
// The first return value is true if any state was actually loaded, false otherwise.
func (s *MCPState) Load(con *libcontext.Context, cfg *config.MCPConfig) (bool, error) {
	debug.Debug("Loading MCP state")
	rawState, err := libstate.LoadState(con.GenericStatePath, con.PluginStatePath)
	if err != nil {
		return false, fmt.Errorf("error loading kubeswitcher state: %w", err)
	}
	loaded, err := DetermineMCPStateFromRawState(con, cfg, rawState)
	if err != nil {
		return false, fmt.Errorf("error determining MCP state from raw state: %w", err)
	}
	if loaded != nil {
		s.copyFrom(loaded)
		debug.Debug("Successfully loaded MCP state")
		return true, nil
	}
	debug.Debug("No MCP state could be loaded")
	return false, nil
}

// DetermineMCPStateFromRawState takes the raw kubeswitcher state and tries to determine the MCP state from it.
// Returns the state, if it came from this plugin and could be recovered.
// Returns an error, if the state came from this plugin, but could not be recovered (e.g. due to an unmarshaling error).
// Returns nil (both return values) if the state did not come from this plugin or could not be loaded.
func DetermineMCPStateFromRawState(con *libcontext.Context, cfg *config.MCPConfig, rawState *libstate.State) (*MCPState, error) {
	if rawState == nil || rawState.LastUsed == nil || rawState.LastUsed.Plugin != con.CurrentPluginName {
		debug.Debug("Unable to load state (does not exist or comes from a different plugin)")
		return nil, nil
	}
	res := &MCPState{
		Focus: NewEmptyFocus(),
	}

	debug.Debug("Last cluster was selected via mcpu plugin, loading state")
	err := yaml.Unmarshal(rawState.RawPluginState, &res)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling plugin state: %w", err)
	}
	pData, err := yaml.Marshal(res)
	if err != nil {
		debug.Debug("Error marshaling loaded state to yaml: %v", err)
	} else {
		debug.Debug("Loaded state from mcp plugin:\n%s", string(pData))
	}

	return res, nil
}
