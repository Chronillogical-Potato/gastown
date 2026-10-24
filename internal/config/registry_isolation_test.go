package config

import (
	"reflect"
	"testing"
)

// agentRegistryIsolationEnv is set via t.Setenv by isolateAgentRegistry. It is
// never read; t.Setenv is used because the testing package panics when a test
// that called it is (or later becomes) parallel, which turns "mutated the
// process-global agent registry from a parallel test" into a loud failure
// instead of an order-dependent flake in unrelated tests.
const agentRegistryIsolationEnv = "GT_TEST_AGENT_REGISTRY_ISOLATED"

// isolateAgentRegistry gives t exclusive use of the process-global agent
// registry: it resets the registry to the built-in presets now and again when
// t finishes. Every test that mutates the registry (RegisterAgentForTesting,
// LoadAgentRegistry, LoadRigAgentRegistry, ResetRegistryForTesting) must call
// it and must not be parallel. Parallel tests only start after all sequential
// tests have returned, so read-only parallel tests always observe the
// built-in presets.
func isolateAgentRegistry(t *testing.T) {
	t.Helper()
	t.Setenv(agentRegistryIsolationEnv, "1")
	ResetRegistryForTesting()
	t.Cleanup(ResetRegistryForTesting)
}

// TestAgentRegistryReturnsIsolatedCopies is a regression test: the registry
// getters used to hand out pointers (and slices) that aliased the registry
// entries, which in turn aliased the package-level builtinPresets. Any caller
// that modified a returned preset silently rewrote the built-in defaults for
// the rest of the process, and ResetRegistryForTesting could not undo it.
func TestAgentRegistryReturnsIsolatedCopies(t *testing.T) {
	isolateAgentRegistry(t)

	pristineCodex := cloneAgentPresetInfo(builtinPresets[AgentCodex])
	pristineOpenCode := cloneAgentPresetInfo(builtinPresets[AgentOpenCode])
	// Restore the built-ins even if the code under test corrupted them, so a
	// failure here does not cascade into unrelated tests.
	t.Cleanup(func() {
		builtinPresets[AgentCodex] = cloneAgentPresetInfo(pristineCodex)
		builtinPresets[AgentOpenCode] = cloneAgentPresetInfo(pristineOpenCode)
	})

	byPreset := GetAgentPreset(AgentCodex)
	byPreset.Command = "mutated"
	byPreset.Args[0] = "mutated"
	byPreset.ProcessNames[0] = "mutated"
	byPreset.NonInteractive.Subcommand = "mutated"

	byName := GetAgentPresetByName(string(AgentCodex))
	byName.Command = "mutated-by-name"

	GetProcessNames(string(AgentCodex))[0] = "mutated"
	ResolveProcessNames(string(AgentCodex), "codex")[0] = "mutated"
	ResolveProcessNames("some-agent", "sudo", "-u", "runner", "codex")[0] = "mutated"

	acp := ResolveACPConfig(string(AgentOpenCode), "opencode")
	if acp == nil {
		t.Fatal("ResolveACPConfig(opencode) = nil, want built-in ACP config")
	}
	acp.Command = "mutated"

	registered := AgentPresetInfo{
		Name:         "custom-tool",
		Command:      "custom-tool",
		ProcessNames: []string{"custom-tool"},
	}
	RegisterAgentForTesting("custom-tool", registered)
	registered.ProcessNames[0] = "mutated"

	assertPreset := func(stage string) {
		t.Helper()
		if got := GetAgentPreset(AgentCodex); !reflect.DeepEqual(got, pristineCodex) {
			t.Errorf("%s: GetAgentPreset(codex) = %+v, want %+v", stage, got, pristineCodex)
		}
		if got := builtinPresets[AgentCodex]; !reflect.DeepEqual(got, pristineCodex) {
			t.Errorf("%s: builtinPresets[codex] = %+v, want %+v", stage, got, pristineCodex)
		}
		if got := GetAgentPreset(AgentOpenCode); !reflect.DeepEqual(got, pristineOpenCode) {
			t.Errorf("%s: GetAgentPreset(opencode) = %+v, want %+v", stage, got, pristineOpenCode)
		}
		if got := GetProcessNames(string(AgentCodex)); !reflect.DeepEqual(got, []string{"codex"}) {
			t.Errorf("%s: GetProcessNames(codex) = %v, want [codex]", stage, got)
		}
	}
	assertPreset("after mutating returned values")

	if got := GetProcessNames("custom-tool"); !reflect.DeepEqual(got, []string{"custom-tool"}) {
		t.Errorf("GetProcessNames(custom-tool) = %v, want [custom-tool]", got)
	}

	ResetRegistryForTesting()
	assertPreset("after ResetRegistryForTesting")
}
