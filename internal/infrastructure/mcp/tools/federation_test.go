// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/infrastructure/federation"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestFederationTools_ImplementsMCPToolProvider(t *testing.T) {
	var _ shared.MCPToolProvider = (*FederationTools)(nil)
}

func TestFederationTools_GetTools(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}
}

func TestFederationTools_GetTools_ExpectedUniqueNames(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}

	expectedNames := map[string]bool{
		"federation/status":              true,
		"federation/spawn-ephemeral":     true,
		"federation/terminate-ephemeral": true,
		"federation/list-ephemeral":      true,
		"federation/register-swarm":      true,
		"federation/broadcast":           true,
		"federation/propose":             true,
		"federation/vote":                true,
	}

	if len(tools) != len(expectedNames) {
		t.Fatalf("expected %d federation tools, got %d", len(expectedNames), len(tools))
	}

	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if seen[tool.Name] {
			t.Fatalf("duplicate federation tool name: %s", tool.Name)
		}
		seen[tool.Name] = true

		if !expectedNames[tool.Name] {
			t.Fatalf("unexpected federation tool name: %s", tool.Name)
		}
	}

	for name := range expectedNames {
		if !seen[name] {
			t.Fatalf("expected federation tool missing: %s", name)
		}
	}
}

func TestFederationTools_GetTools_HaveObjectSchemasAndRequiredFields(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	if len(tools) == 0 {
		t.Fatal("expected federation tools to be registered")
	}

	type requiredExpectation struct {
		fields map[string]bool
	}
	expectedRequired := map[string]requiredExpectation{
		"federation/spawn-ephemeral":     {fields: map[string]bool{"type": true, "task": true}},
		"federation/terminate-ephemeral": {fields: map[string]bool{"agentId": true}},
		"federation/register-swarm":      {fields: map[string]bool{"swarmId": true, "name": true, "maxAgents": true}},
		"federation/broadcast":           {fields: map[string]bool{"sourceSwarmId": true, "payload": true}},
		"federation/propose":             {fields: map[string]bool{"proposerId": true, "proposalType": true, "value": true}},
		"federation/vote":                {fields: map[string]bool{"voterId": true, "proposalId": true, "approve": true}},
	}

	for _, tool := range tools {
		if tool.Description == "" {
			t.Fatalf("tool %s should have a non-empty description", tool.Name)
		}

		if tool.Parameters["type"] != "object" {
			t.Fatalf("tool %s should use an object schema, got %v", tool.Name, tool.Parameters["type"])
		}

		expectation, needsRequired := expectedRequired[tool.Name]
		if !needsRequired {
			continue
		}

		rawRequired, ok := tool.Parameters["required"]
		if !ok {
			t.Fatalf("tool %s should define required fields", tool.Name)
		}

		requiredList, ok := rawRequired.([]string)
		if !ok {
			t.Fatalf("tool %s required fields should be []string, got %T", tool.Name, rawRequired)
		}

		if len(requiredList) != len(expectation.fields) {
			t.Fatalf("tool %s expected %d required fields, got %d", tool.Name, len(expectation.fields), len(requiredList))
		}

		seen := make(map[string]bool, len(requiredList))
		for _, field := range requiredList {
			seen[field] = true
		}

		for field := range expectation.fields {
			if !seen[field] {
				t.Fatalf("tool %s missing required field %q", tool.Name, field)
			}
		}
	}
}

func TestFederationTools_GetTools_ListEphemeralStatusEnum(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	var listTool *shared.MCPTool
	for i := range tools {
		if tools[i].Name == "federation/list-ephemeral" {
			listTool = &tools[i]
			break
		}
	}
	if listTool == nil {
		t.Fatal("expected federation/list-ephemeral tool to exist")
	}

	propertiesRaw, ok := listTool.Parameters["properties"]
	if !ok {
		t.Fatal("expected properties in list-ephemeral schema")
	}
	properties, ok := propertiesRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties to be map[string]interface{}, got %T", propertiesRaw)
	}

	statusRaw, ok := properties["status"]
	if !ok {
		t.Fatal("expected status property in list-ephemeral schema")
	}
	statusProperty, ok := statusRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status property map, got %T", statusRaw)
	}

	enumRaw, ok := statusProperty["enum"]
	if !ok {
		t.Fatal("expected enum definition for list-ephemeral status property")
	}
	enumValues, ok := enumRaw.([]string)
	if !ok {
		t.Fatalf("expected status enum as []string, got %T", enumRaw)
	}

	expected := map[string]bool{
		"spawning":   true,
		"active":     true,
		"completing": true,
		"terminated": true,
	}
	if len(enumValues) != len(expected) {
		t.Fatalf("expected %d enum values, got %d (%v)", len(expected), len(enumValues), enumValues)
	}
	for _, value := range enumValues {
		if !expected[value] {
			t.Fatalf("unexpected enum value %q in status property", value)
		}
		delete(expected, value)
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected enum values: %v", expected)
	}
}

func TestFederationTools_GetTools_IntegerNumericParameters(t *testing.T) {
	ft := &FederationTools{}

	tools := ft.GetTools()
	toolByName := make(map[string]shared.MCPTool, len(tools))
	for _, tool := range tools {
		toolByName[tool.Name] = tool
	}

	assertPropertyType := func(toolName, propertyName, expectedType string) {
		t.Helper()

		tool, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := tool.Parameters["properties"]
		if !ok {
			t.Fatalf("expected properties for tool %s", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for tool %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s on tool %s", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		if property["type"] != expectedType {
			t.Fatalf("expected %s.%s type %q, got %v", toolName, propertyName, expectedType, property["type"])
		}
	}

	assertPropertyNumber := func(toolName, propertyName, key string, expected float64) {
		t.Helper()

		tool, ok := toolByName[toolName]
		if !ok {
			t.Fatalf("expected tool %s to be present", toolName)
		}
		propertiesRaw, ok := tool.Parameters["properties"]
		if !ok {
			t.Fatalf("expected properties for tool %s", toolName)
		}
		properties, ok := propertiesRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected properties map for tool %s, got %T", toolName, propertiesRaw)
		}
		propertyRaw, ok := properties[propertyName]
		if !ok {
			t.Fatalf("expected property %s on tool %s", propertyName, toolName)
		}
		property, ok := propertyRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("expected property map for %s.%s, got %T", toolName, propertyName, propertyRaw)
		}
		valueRaw, ok := property[key]
		if !ok {
			t.Fatalf("expected %s.%s to define %s", toolName, propertyName, key)
		}
		value, ok := valueRaw.(float64)
		if !ok {
			t.Fatalf("expected %s.%s %s to be float64, got %T", toolName, propertyName, key, valueRaw)
		}
		if value != expected {
			t.Fatalf("expected %s.%s %s %v, got %v", toolName, propertyName, key, expected, value)
		}
	}

	assertPropertyType("federation/spawn-ephemeral", "ttl", "integer")
	assertPropertyType("federation/register-swarm", "maxAgents", "integer")
	assertPropertyNumber("federation/spawn-ephemeral", "ttl", "minimum", 1)
	assertPropertyNumber("federation/spawn-ephemeral", "ttl", "maximum", float64(math.MaxInt64))
	assertPropertyNumber("federation/register-swarm", "maxAgents", "minimum", 1)
	assertPropertyNumber("federation/register-swarm", "maxAgents", "maximum", float64(math.MaxInt))
}

func TestNormalizeCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "string slice trims and deduplicates",
			input:    []string{"go", " docker ", "go", "", "   ", "docker"},
			expected: []string{"go", "docker"},
		},
		{
			name:     "interface slice trims filters and deduplicates",
			input:    []interface{}{"gpu", " gpu ", "", "  ", 42, nil, "ml", "gpu"},
			expected: []string{"gpu", "ml"},
		},
		{
			name:     "unsupported input type",
			input:    "gpu",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCapabilities(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d capabilities, got %d (%v)", len(tt.expected), len(got), got)
			}
			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Fatalf("capability mismatch at index %d: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestValidateCapabilitiesInput(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		shouldErr bool
	}{
		{name: "string slice", input: []string{"go", "docker"}, shouldErr: false},
		{name: "interface string slice", input: []interface{}{"go", "docker"}, shouldErr: false},
		{name: "nil input", input: nil, shouldErr: true},
		{name: "non-array", input: "go", shouldErr: true},
		{name: "interface slice with non-string", input: []interface{}{"go", 1}, shouldErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCapabilitiesInput(tt.input)
			if tt.shouldErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Fatalf("expected validation success, got error: %v", err)
			}
		})
	}
}

func TestParseEphemeralAgentStatus(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expected     shared.EphemeralAgentStatus
		expectValid  bool
	}{
		{
			name:        "valid lowercase",
			input:       "active",
			expected:    shared.EphemeralStatusActive,
			expectValid: true,
		},
		{
			name:        "valid uppercase with padding",
			input:       "  TERMINATED  ",
			expected:    shared.EphemeralStatusTerminated,
			expectValid: true,
		},
		{
			name:        "invalid value",
			input:       "paused",
			expectValid: false,
		},
		{
			name:        "blank",
			input:       "   ",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseEphemeralAgentStatus(tt.input)
			if ok != tt.expectValid {
				t.Fatalf("expected valid=%v, got %v (status=%q)", tt.expectValid, ok, got)
			}
			if tt.expectValid && got != tt.expected {
				t.Fatalf("expected status %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCloneInterfaceValue_DeepCloneBehavior(t *testing.T) {
	original := map[string]interface{}{
		"meta": map[string]interface{}{
			"nested": map[string]interface{}{"k": "v"},
			"tags":   map[string]string{"owner": "team-a"},
			"flags":  map[string]bool{"featureA": true},
			"limits": map[string]int{"max": 10},
			"quota":  map[string]int64{"hard": 20},
		},
		"items": []interface{}{
			map[string]interface{}{"id": "a"},
			[]interface{}{map[string]string{"kind": "x"}, map[string]bool{"ready": true}},
		},
		"counts": []int{1, 2, 3},
		"longCounts": []int64{7, 8, 9},
		"ratios": []float64{0.5, 0.75},
		"stringMaps": []map[string]string{
			{"env": "prod"},
		},
		"ifaceMaps": []map[string]interface{}{
			{"status": "active"},
		},
		"boolMaps": []map[string]bool{
			{"enabled": true},
		},
		"intMaps": []map[string]int{
			{"max": 10},
		},
		"int64Maps": []map[string]int64{
			{"hard": 20},
		},
	}

	clonedRaw := cloneInterfaceValue(original)
	cloned, ok := clonedRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected cloned type map[string]interface{}, got %T", clonedRaw)
	}
	if !reflect.DeepEqual(original, cloned) {
		t.Fatalf("expected cloned value to match original before mutation; original=%v cloned=%v", original, cloned)
	}

	clonedMeta := cloned["meta"].(map[string]interface{})
	clonedMeta["nested"].(map[string]interface{})["k"] = "mutated"
	clonedMeta["tags"].(map[string]string)["owner"] = "mutated-owner"
	clonedMeta["flags"].(map[string]bool)["featureA"] = false
	clonedMeta["limits"].(map[string]int)["max"] = 999
	clonedMeta["quota"].(map[string]int64)["hard"] = 999

	clonedItems := cloned["items"].([]interface{})
	clonedItems[0].(map[string]interface{})["id"] = "mutated-id"
	clonedItems[1].([]interface{})[0].(map[string]string)["kind"] = "mutated-kind"
	clonedItems[1].([]interface{})[1].(map[string]bool)["ready"] = false

	cloned["counts"].([]int)[0] = 100
	cloned["longCounts"].([]int64)[0] = 700
	cloned["ratios"].([]float64)[0] = 9.9
	cloned["stringMaps"].([]map[string]string)[0]["env"] = "staging"
	cloned["ifaceMaps"].([]map[string]interface{})[0]["status"] = "inactive"
	cloned["boolMaps"].([]map[string]bool)[0]["enabled"] = false
	cloned["intMaps"].([]map[string]int)[0]["max"] = 999
	cloned["int64Maps"].([]map[string]int64)[0]["hard"] = 999

	origMeta := original["meta"].(map[string]interface{})
	if origMeta["nested"].(map[string]interface{})["k"] != "v" {
		t.Fatalf("expected original nested map to remain unchanged, got %v", origMeta["nested"].(map[string]interface{})["k"])
	}
	if origMeta["tags"].(map[string]string)["owner"] != "team-a" {
		t.Fatalf("expected original typed tags map to remain unchanged, got %v", origMeta["tags"].(map[string]string)["owner"])
	}
	if origMeta["flags"].(map[string]bool)["featureA"] != true {
		t.Fatalf("expected original typed flags map to remain unchanged, got %v", origMeta["flags"].(map[string]bool)["featureA"])
	}
	if origMeta["limits"].(map[string]int)["max"] != 10 {
		t.Fatalf("expected original typed limits map to remain unchanged, got %v", origMeta["limits"].(map[string]int)["max"])
	}
	if origMeta["quota"].(map[string]int64)["hard"] != 20 {
		t.Fatalf("expected original typed int64 quota map to remain unchanged, got %v", origMeta["quota"].(map[string]int64)["hard"])
	}

	origItems := original["items"].([]interface{})
	if origItems[0].(map[string]interface{})["id"] != "a" {
		t.Fatalf("expected original items map to remain unchanged, got %v", origItems[0].(map[string]interface{})["id"])
	}
	if origItems[1].([]interface{})[0].(map[string]string)["kind"] != "x" {
		t.Fatalf("expected original nested typed map in slice to remain unchanged, got %v", origItems[1].([]interface{})[0].(map[string]string)["kind"])
	}
	if origItems[1].([]interface{})[1].(map[string]bool)["ready"] != true {
		t.Fatalf("expected original nested typed bool map in slice to remain unchanged, got %v", origItems[1].([]interface{})[1].(map[string]bool)["ready"])
	}

	if original["stringMaps"].([]map[string]string)[0]["env"] != "prod" {
		t.Fatalf("expected original []map[string]string entry to remain unchanged, got %v", original["stringMaps"].([]map[string]string)[0]["env"])
	}
	if original["ifaceMaps"].([]map[string]interface{})[0]["status"] != "active" {
		t.Fatalf("expected original []map[string]interface{} entry to remain unchanged, got %v", original["ifaceMaps"].([]map[string]interface{})[0]["status"])
	}
	if original["boolMaps"].([]map[string]bool)[0]["enabled"] != true {
		t.Fatalf("expected original []map[string]bool entry to remain unchanged, got %v", original["boolMaps"].([]map[string]bool)[0]["enabled"])
	}
	if original["counts"].([]int)[0] != 1 {
		t.Fatalf("expected original []int entry to remain unchanged, got %v", original["counts"].([]int)[0])
	}
	if original["longCounts"].([]int64)[0] != 7 {
		t.Fatalf("expected original []int64 entry to remain unchanged, got %v", original["longCounts"].([]int64)[0])
	}
	if original["ratios"].([]float64)[0] != 0.5 {
		t.Fatalf("expected original []float64 entry to remain unchanged, got %v", original["ratios"].([]float64)[0])
	}
	if original["intMaps"].([]map[string]int)[0]["max"] != 10 {
		t.Fatalf("expected original []map[string]int entry to remain unchanged, got %v", original["intMaps"].([]map[string]int)[0]["max"])
	}
	if original["int64Maps"].([]map[string]int64)[0]["hard"] != 20 {
		t.Fatalf("expected original []map[string]int64 entry to remain unchanged, got %v", original["int64Maps"].([]map[string]int64)[0]["hard"])
	}
}

func TestCloneFederationMessage_DeepCopiesPayload(t *testing.T) {
	original := &shared.FederationMessage{
		ID:            "msg-1",
		Type:          shared.FederationMsgBroadcast,
		SourceSwarmID: "swarm-a",
		Payload: map[string]interface{}{
			"event": "created",
			"meta":  map[string]interface{}{"v": "1"},
			"tags":  map[string]string{"env": "prod"},
			"flags": map[string]bool{"enabled": true},
		},
	}

	cloned := cloneFederationMessage(original)
	if cloned == nil {
		t.Fatal("expected cloned message")
	}
	if cloned == original {
		t.Fatal("expected cloned message pointer to differ from original")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("expected cloned message to equal original before mutation; original=%v cloned=%v", original, cloned)
	}

	clonedPayload := cloned.Payload.(map[string]interface{})
	clonedPayload["event"] = "mutated"
	clonedPayload["meta"].(map[string]interface{})["v"] = "2"
	clonedPayload["tags"].(map[string]string)["env"] = "staging"
	clonedPayload["flags"].(map[string]bool)["enabled"] = false

	origPayload := original.Payload.(map[string]interface{})
	if origPayload["event"] != "created" {
		t.Fatalf("expected original payload event unchanged, got %v", origPayload["event"])
	}
	if origPayload["meta"].(map[string]interface{})["v"] != "1" {
		t.Fatalf("expected original nested payload unchanged, got %v", origPayload["meta"].(map[string]interface{})["v"])
	}
	if origPayload["tags"].(map[string]string)["env"] != "prod" {
		t.Fatalf("expected original typed tags unchanged, got %v", origPayload["tags"].(map[string]string)["env"])
	}
	if origPayload["flags"].(map[string]bool)["enabled"] != true {
		t.Fatalf("expected original typed flags unchanged, got %v", origPayload["flags"].(map[string]bool)["enabled"])
	}
}

func TestCloneFederationProposal_DeepCopiesValueAndVotes(t *testing.T) {
	original := &shared.FederationProposal{
		ID:         "proposal-1",
		ProposerID: "swarm-a",
		Type:       "scale",
		Value: map[string]interface{}{
			"config": map[string]interface{}{"maxAgents": float64(5)},
			"tags":   map[string]string{"priority": "high"},
			"flags":  map[string]bool{"allowScale": true},
		},
		Votes: map[string]bool{
			"swarm-a": true,
		},
		Status: shared.FederationProposalPending,
	}

	cloned := cloneFederationProposal(original)
	if cloned == nil {
		t.Fatal("expected cloned proposal")
	}
	if cloned == original {
		t.Fatal("expected cloned proposal pointer to differ from original")
	}
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("expected cloned proposal to equal original before mutation; original=%v cloned=%v", original, cloned)
	}

	clonedValue := cloned.Value.(map[string]interface{})
	clonedValue["config"].(map[string]interface{})["maxAgents"] = float64(99)
	clonedValue["tags"].(map[string]string)["priority"] = "low"
	clonedValue["flags"].(map[string]bool)["allowScale"] = false
	cloned.Votes["swarm-b"] = false

	origValue := original.Value.(map[string]interface{})
	if origValue["config"].(map[string]interface{})["maxAgents"] != float64(5) {
		t.Fatalf("expected original config unchanged, got %v", origValue["config"].(map[string]interface{})["maxAgents"])
	}
	if origValue["tags"].(map[string]string)["priority"] != "high" {
		t.Fatalf("expected original typed tags unchanged, got %v", origValue["tags"].(map[string]string)["priority"])
	}
	if origValue["flags"].(map[string]bool)["allowScale"] != true {
		t.Fatalf("expected original typed flags unchanged, got %v", origValue["flags"].(map[string]bool)["allowScale"])
	}
	if _, exists := original.Votes["swarm-b"]; exists {
		t.Fatal("expected original votes map unchanged")
	}
}

func TestSortEphemeralAgents_HandlesNilEntries(t *testing.T) {
	agents := []*shared.EphemeralAgent{
		nil,
		{ID: "b", CreatedAt: 200},
		nil,
		{ID: "a", CreatedAt: 100},
		{ID: "c", CreatedAt: 200},
	}

	sortEphemeralAgents(agents)

	expectedOrder := []string{"a", "b", "c"}
	for i, expectedID := range expectedOrder {
		if agents[i] == nil {
			t.Fatalf("expected non-nil agent at index %d", i)
		}
		if agents[i].ID != expectedID {
			t.Fatalf("expected agent ID %q at index %d, got %q", expectedID, i, agents[i].ID)
		}
	}
	for i := len(expectedOrder); i < len(agents); i++ {
		if agents[i] != nil {
			t.Fatalf("expected nil agent entries to be sorted to tail, found non-nil at index %d: %v", i, agents[i])
		}
	}
}

func TestFederationTools_Execute_UnknownTool(t *testing.T) {
	ft := &FederationTools{}

	result, err := ft.Execute(context.Background(), "federation/unknown-tool", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Fatal("unknown tool should not succeed")
	}
	if result.Error == "" {
		t.Fatal("expected error message in tool result")
	}
}

func TestFederationTools_Execute_Status(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	result, err := ft.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if err != nil {
		t.Fatalf("expected status tool to succeed, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Success {
		t.Fatalf("expected successful result, got error: %s", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected status data map, got %T", result.Data)
	}
	if _, ok := data["federationId"]; !ok {
		t.Fatal("expected federationId in status response")
	}
	if _, ok := data["stats"]; !ok {
		t.Fatal("expected stats in status response")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_StatusParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/status", map[string]interface{}{})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected Execute and ExecuteTool success parity, got %v vs %v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool data map, got %T", directResult.Data)
	}

	if _, ok := execData["federationId"]; !ok {
		t.Fatal("expected Execute data to contain federationId")
	}
	if _, ok := directData["federationId"]; !ok {
		t.Fatal("expected ExecuteTool data to contain federationId")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_StatusDeterministicSwarmOrdering(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-zeta",
		Name:      "Zeta",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm-zeta: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-alpha",
		Name:      "Alpha",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm-alpha: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-beta",
		Name:      "Beta",
		MaxAgents: 3,
	}); err != nil {
		t.Fatalf("failed to register swarm-beta: %v", err)
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/status", map[string]interface{}{})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/status", map[string]interface{}{})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool data map, got %T", directResult.Data)
	}

	execSwarmsRaw, ok := execData["swarms"]
	if !ok {
		t.Fatal("expected Execute status data to contain swarms")
	}
	directSwarmsRaw, ok := directData["swarms"]
	if !ok {
		t.Fatal("expected ExecuteTool status data to contain swarms")
	}

	execSwarms, ok := execSwarmsRaw.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute swarms as []map[string]interface{}, got %T", execSwarmsRaw)
	}
	directSwarms, ok := directSwarmsRaw.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool swarms as []map[string]interface{}, got %T", directSwarmsRaw)
	}

	if len(execSwarms) != len(directSwarms) {
		t.Fatalf("expected swarm list parity, got Execute=%d ExecuteTool=%d", len(execSwarms), len(directSwarms))
	}

	for i := 1; i < len(execSwarms); i++ {
		prev := execSwarms[i-1]["swarmId"].(string)
		curr := execSwarms[i]["swarmId"].(string)
		if prev > curr {
			t.Fatalf("expected Execute swarm order to be sorted ascending, got %q before %q", prev, curr)
		}
	}
	for i := range execSwarms {
		execID := execSwarms[i]["swarmId"]
		directID := directSwarms[i]["swarmId"]
		if execID != directID {
			t.Fatalf("expected deterministic swarm ordering parity at index %d, got Execute=%v ExecuteTool=%v", i, execID, directID)
		}
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}

	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralDeterministicOrderParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-order", Name: "Order Swarm", MaxAgents: 10}); err != nil {
		t.Fatalf("failed to register order swarm: %v", err)
	}

	for i := 0; i < 4; i++ {
		if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
			SwarmID: "swarm-list-order",
			Type:    "coder",
			Task:    "ordered listing parity",
		}); err != nil {
			t.Fatalf("failed to spawn list-order agent %d: %v", i, err)
		}
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", map[string]interface{}{})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", map[string]interface{}{})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}
	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
	if len(execAgents) < 4 {
		t.Fatalf("expected at least 4 list entries, got %d", len(execAgents))
	}

	assertSorted := func(agents []*shared.EphemeralAgent, label string) {
		for i := 1; i < len(agents); i++ {
			prev := agents[i-1]
			curr := agents[i]
			if prev.CreatedAt > curr.CreatedAt {
				t.Fatalf("%s list is not sorted by CreatedAt at index %d (%d > %d)", label, i, prev.CreatedAt, curr.CreatedAt)
			}
			if prev.CreatedAt == curr.CreatedAt && prev.ID > curr.ID {
				t.Fatalf("%s list tie-break sort by ID violated at index %d (%s > %s)", label, i, prev.ID, curr.ID)
			}
		}
	}

	assertSorted(execAgents, "Execute")
	assertSorted(directAgents, "ExecuteTool")

	for i := range execAgents {
		if execAgents[i].ID != directAgents[i].ID {
			t.Fatalf("expected deterministic order parity at index %d, got Execute=%s ExecuteTool=%s", i, execAgents[i].ID, directAgents[i].ID)
		}
	}
}

func TestFederationTools_ListEphemeralReturnsDefensiveCopies(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-defensive-copy",
		Name:      "Defensive Copy Swarm",
		MaxAgents: 5,
	}); err != nil {
		t.Fatalf("failed to register swarm: %v", err)
	}

	spawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-defensive-copy",
		Type:    "coder",
		Task:    "original task",
		Metadata: map[string]interface{}{
			"role":   "original",
			"nested": map[string]interface{}{"k": "v"},
			"tags":   map[string]string{"owner": "team-a"},
		},
	})
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}
	if err := hub.CompleteAgent(spawn.AgentID, map[string]interface{}{
		"outcome": map[string]interface{}{"score": float64(1)},
	}); err != nil {
		t.Fatalf("failed to complete agent: %v", err)
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", map[string]interface{}{
		"swarmId": "swarm-defensive-copy",
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	var execListed *shared.EphemeralAgent
	for _, agent := range execAgents {
		if agent.ID == spawn.AgentID {
			execListed = agent
			break
		}
	}
	if execListed == nil {
		t.Fatalf("expected Execute list to include spawned agent %s", spawn.AgentID)
	}
	execListed.Task = "mutated via execute"
	if execListed.Metadata == nil {
		execListed.Metadata = map[string]interface{}{}
	}
	execListed.Metadata["role"] = "mutated-execute"
	nestedExec, ok := execListed.Metadata["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute nested metadata map, got %T", execListed.Metadata["nested"])
	}
	nestedExec["k"] = "mutated-execute-nested"
	execTags, ok := execListed.Metadata["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected Execute tags metadata map[string]string, got %T", execListed.Metadata["tags"])
	}
	execTags["owner"] = "team-exec"
	execResultMap, ok := execListed.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute result map, got %T", execListed.Result)
	}
	execOutcome, ok := execResultMap["outcome"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute nested outcome map, got %T", execResultMap["outcome"])
	}
	execOutcome["score"] = float64(99)

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", map[string]interface{}{
		"swarmId": "swarm-defensive-copy",
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}
	var directListed *shared.EphemeralAgent
	for _, agent := range directAgents {
		if agent.ID == spawn.AgentID {
			directListed = agent
			break
		}
	}
	if directListed == nil {
		t.Fatalf("expected ExecuteTool list to include spawned agent %s", spawn.AgentID)
	}
	directListed.Task = "mutated via executeTool"
	if directListed.Metadata == nil {
		directListed.Metadata = map[string]interface{}{}
	}
	directListed.Metadata["role"] = "mutated-direct"
	nestedDirect, ok := directListed.Metadata["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool nested metadata map, got %T", directListed.Metadata["nested"])
	}
	nestedDirect["k"] = "mutated-direct-nested"
	directTags, ok := directListed.Metadata["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected ExecuteTool tags metadata map[string]string, got %T", directListed.Metadata["tags"])
	}
	directTags["owner"] = "team-direct"
	directResultMap, ok := directListed.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool result map, got %T", directListed.Result)
	}
	directOutcome, ok := directResultMap["outcome"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool nested outcome map, got %T", directResultMap["outcome"])
	}
	directOutcome["score"] = float64(123)

	stored, ok := hub.GetAgent(spawn.AgentID)
	if !ok {
		t.Fatalf("expected spawned agent %s to still exist", spawn.AgentID)
	}
	if stored.Task != "original task" {
		t.Fatalf("expected hub state task to remain original, got %q", stored.Task)
	}
	if stored.Metadata["role"] != "original" {
		t.Fatalf("expected hub metadata role to remain original, got %v", stored.Metadata["role"])
	}
	storedNested, ok := stored.Metadata["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored nested metadata map, got %T", stored.Metadata["nested"])
	}
	if storedNested["k"] != "v" {
		t.Fatalf("expected stored nested metadata to remain original, got %v", storedNested["k"])
	}
	storedTags, ok := stored.Metadata["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected stored tags metadata map[string]string, got %T", stored.Metadata["tags"])
	}
	if storedTags["owner"] != "team-a" {
		t.Fatalf("expected stored tags owner to remain team-a, got %v", storedTags["owner"])
	}
	storedResult, ok := stored.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored result map, got %T", stored.Result)
	}
	storedOutcome, ok := storedResult["outcome"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored outcome map, got %T", storedResult["outcome"])
	}
	if storedOutcome["score"] != float64(1) {
		t.Fatalf("expected stored outcome score to remain original, got %v", storedOutcome["score"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralSwarmFilterParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-target", Name: "Target", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-other", Name: "Other", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register other swarm: %v", err)
	}

	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-target",
		Type:    "coder",
		Task:    "target task",
	}); err != nil {
		t.Fatalf("failed to spawn target agent: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-other",
		Type:    "reviewer",
		Task:    "other task",
	}); err != nil {
		t.Fatalf("failed to spawn other agent: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"swarmId": "  swarm-list-target  ",
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed with swarm filter, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with swarm filter, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}
	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
	if len(execAgents) == 0 {
		t.Fatal("expected at least one filtered agent for target swarm")
	}
	for _, agent := range execAgents {
		if agent.SwarmID != "swarm-list-target" {
			t.Fatalf("expected Execute swarm filter to return only swarm-list-target agents, got %q", agent.SwarmID)
		}
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralStatusFilterParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-status", Name: "Status Swarm", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register status swarm: %v", err)
	}

	terminatedSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-status",
		Type:    "coder",
		Task:    "terminate me",
	})
	if err != nil {
		t.Fatalf("failed to spawn terminated candidate agent: %v", err)
	}
	if err := hub.TerminateAgent(terminatedSpawn.AgentID, "done"); err != nil {
		t.Fatalf("failed to terminate candidate agent: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-status",
		Type:    "reviewer",
		Task:    "stay alive",
	}); err != nil {
		t.Fatalf("failed to spawn non-terminated agent: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"status": "  terminated  ",
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed with status filter, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with status filter, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}
	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
	if len(execAgents) == 0 {
		t.Fatal("expected at least one terminated agent")
	}
	for _, agent := range execAgents {
		if agent.Status != shared.EphemeralStatusTerminated {
			t.Fatalf("expected terminated status filter to return only terminated agents, got %q", agent.Status)
		}
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralCombinedFilterParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-combined-target", Name: "Combined Target", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-combined-other", Name: "Combined Other", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register other swarm: %v", err)
	}

	targetSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-combined-target",
		Type:    "coder",
		Task:    "target terminated",
	})
	if err != nil {
		t.Fatalf("failed to spawn target agent: %v", err)
	}
	if err := hub.TerminateAgent(targetSpawn.AgentID, "done"); err != nil {
		t.Fatalf("failed to terminate target agent: %v", err)
	}

	otherSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-combined-other",
		Type:    "reviewer",
		Task:    "other terminated",
	})
	if err != nil {
		t.Fatalf("failed to spawn other agent: %v", err)
	}
	if err := hub.TerminateAgent(otherSpawn.AgentID, "done"); err != nil {
		t.Fatalf("failed to terminate other agent: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"swarmId": "  swarm-list-combined-target  ",
		"status":  "  TERMINATED  ",
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed with combined filters, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with combined filters, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execAgents, ok := execResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected Execute data type []*shared.EphemeralAgent, got %T", execResult.Data)
	}
	directAgents, ok := directResult.Data.([]*shared.EphemeralAgent)
	if !ok {
		t.Fatalf("expected ExecuteTool data type []*shared.EphemeralAgent, got %T", directResult.Data)
	}
	if len(execAgents) != len(directAgents) {
		t.Fatalf("expected list parity, got Execute=%d ExecuteTool=%d", len(execAgents), len(directAgents))
	}
	if len(execAgents) == 0 {
		t.Fatal("expected at least one combined-filter match")
	}
	for _, agent := range execAgents {
		if agent.SwarmID != "swarm-list-combined-target" {
			t.Fatalf("expected combined filter to constrain swarm, got %q", agent.SwarmID)
		}
		if agent.Status != shared.EphemeralStatusTerminated {
			t.Fatalf("expected combined filter to constrain status, got %q", agent.Status)
		}
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralBlankFiltersNoopParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-a", Name: "Blank A", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register blank swarm A: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-b", Name: "Blank B", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register blank swarm B: %v", err)
	}

	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-a",
		Type:    "coder",
		Task:    "blank filter agent A",
	}); err != nil {
		t.Fatalf("failed to spawn blank filter agent A: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-b",
		Type:    "reviewer",
		Task:    "blank filter agent B",
	}); err != nil {
		t.Fatalf("failed to spawn blank filter agent B: %v", err)
	}

	ft := NewFederationTools(hub)
	blankArgs := map[string]interface{}{
		"swarmId": "   ",
		"status":  "   ",
	}

	baselineExec, baselineExecErr := ft.Execute(context.Background(), "federation/list-ephemeral", map[string]interface{}{})
	if baselineExecErr != nil {
		t.Fatalf("baseline Execute without filters should succeed, got error: %v", baselineExecErr)
	}
	baselineDirect, baselineDirectErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", map[string]interface{}{})
	if baselineDirectErr != nil {
		t.Fatalf("baseline ExecuteTool without filters should succeed, got error: %v", baselineDirectErr)
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", blankArgs)
	if execErr != nil {
		t.Fatalf("Execute should treat blank filters as no-op, got error: %v", execErr)
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", blankArgs)
	if directErr != nil {
		t.Fatalf("ExecuteTool should treat blank filters as no-op, got error: %v", directErr)
	}

	extractIDs := func(result shared.MCPToolResult) []string {
		t.Helper()
		agents, ok := result.Data.([]*shared.EphemeralAgent)
		if !ok {
			t.Fatalf("expected result data type []*shared.EphemeralAgent, got %T", result.Data)
		}
		ids := make([]string, 0, len(agents))
		for _, agent := range agents {
			if agent == nil {
				continue
			}
			ids = append(ids, agent.ID)
		}
		return ids
	}

	baselineExecIDs := extractIDs(*baselineExec)
	baselineDirectIDs := extractIDs(baselineDirect)
	execIDs := extractIDs(*execResult)
	directIDs := extractIDs(directResult)

	if !reflect.DeepEqual(execIDs, baselineExecIDs) {
		t.Fatalf("expected Execute blank-filter IDs %v to match baseline no-filter IDs %v", execIDs, baselineExecIDs)
	}
	if !reflect.DeepEqual(directIDs, baselineDirectIDs) {
		t.Fatalf("expected ExecuteTool blank-filter IDs %v to match baseline no-filter IDs %v", directIDs, baselineDirectIDs)
	}
	if !reflect.DeepEqual(execIDs, directIDs) {
		t.Fatalf("expected Execute and ExecuteTool blank-filter ID parity, got Execute=%v ExecuteTool=%v", execIDs, directIDs)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralBlankStatusUsesSwarmFilterParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-status-target", Name: "Blank Status Target", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-status-other", Name: "Blank Status Other", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register other swarm: %v", err)
	}

	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-status-target",
		Type:    "coder",
		Task:    "target agent A",
	}); err != nil {
		t.Fatalf("failed to spawn target agent A: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-status-target",
		Type:    "reviewer",
		Task:    "target agent B",
	}); err != nil {
		t.Fatalf("failed to spawn target agent B: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-status-other",
		Type:    "analyst",
		Task:    "other agent",
	}); err != nil {
		t.Fatalf("failed to spawn other agent: %v", err)
	}

	ft := NewFederationTools(hub)
	blankStatusArgs := map[string]interface{}{
		"swarmId": "swarm-list-blank-status-target",
		"status":  "   ",
	}
	swarmOnlyArgs := map[string]interface{}{
		"swarmId": "swarm-list-blank-status-target",
	}

	baselineExec, baselineExecErr := ft.Execute(context.Background(), "federation/list-ephemeral", swarmOnlyArgs)
	if baselineExecErr != nil {
		t.Fatalf("baseline Execute with swarm filter should succeed, got error: %v", baselineExecErr)
	}
	baselineDirect, baselineDirectErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", swarmOnlyArgs)
	if baselineDirectErr != nil {
		t.Fatalf("baseline ExecuteTool with swarm filter should succeed, got error: %v", baselineDirectErr)
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", blankStatusArgs)
	if execErr != nil {
		t.Fatalf("Execute should treat blank status as no-op when swarm filter is present, got error: %v", execErr)
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", blankStatusArgs)
	if directErr != nil {
		t.Fatalf("ExecuteTool should treat blank status as no-op when swarm filter is present, got error: %v", directErr)
	}

	extractIDs := func(result shared.MCPToolResult) []string {
		t.Helper()
		agents, ok := result.Data.([]*shared.EphemeralAgent)
		if !ok {
			t.Fatalf("expected result data type []*shared.EphemeralAgent, got %T", result.Data)
		}
		ids := make([]string, 0, len(agents))
		for _, agent := range agents {
			if agent == nil {
				continue
			}
			if agent.SwarmID != "swarm-list-blank-status-target" {
				t.Fatalf("expected only target swarm agents, got %q", agent.SwarmID)
			}
			ids = append(ids, agent.ID)
		}
		return ids
	}

	baselineExecIDs := extractIDs(*baselineExec)
	baselineDirectIDs := extractIDs(baselineDirect)
	execIDs := extractIDs(*execResult)
	directIDs := extractIDs(directResult)

	if !reflect.DeepEqual(execIDs, baselineExecIDs) {
		t.Fatalf("expected Execute blank-status IDs %v to match swarm-only baseline IDs %v", execIDs, baselineExecIDs)
	}
	if !reflect.DeepEqual(directIDs, baselineDirectIDs) {
		t.Fatalf("expected ExecuteTool blank-status IDs %v to match swarm-only baseline IDs %v", directIDs, baselineDirectIDs)
	}
	if !reflect.DeepEqual(execIDs, directIDs) {
		t.Fatalf("expected Execute and ExecuteTool blank-status ID parity, got Execute=%v ExecuteTool=%v", execIDs, directIDs)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralBlankSwarmUsesStatusFilterParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-swarm-a", Name: "Blank Swarm A", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register swarm A: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{SwarmID: "swarm-list-blank-swarm-b", Name: "Blank Swarm B", MaxAgents: 5}); err != nil {
		t.Fatalf("failed to register swarm B: %v", err)
	}

	terminatedA, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-swarm-a",
		Type:    "coder",
		Task:    "terminated A",
	})
	if err != nil {
		t.Fatalf("failed to spawn terminated A candidate: %v", err)
	}
	if err := hub.TerminateAgent(terminatedA.AgentID, "done"); err != nil {
		t.Fatalf("failed to terminate A candidate: %v", err)
	}
	terminatedB, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-swarm-b",
		Type:    "reviewer",
		Task:    "terminated B",
	})
	if err != nil {
		t.Fatalf("failed to spawn terminated B candidate: %v", err)
	}
	if err := hub.TerminateAgent(terminatedB.AgentID, "done"); err != nil {
		t.Fatalf("failed to terminate B candidate: %v", err)
	}
	if _, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-list-blank-swarm-b",
		Type:    "analyst",
		Task:    "active B",
	}); err != nil {
		t.Fatalf("failed to spawn active B agent: %v", err)
	}

	ft := NewFederationTools(hub)
	blankSwarmArgs := map[string]interface{}{
		"swarmId": "   ",
		"status":  "terminated",
	}
	statusOnlyArgs := map[string]interface{}{
		"status": "terminated",
	}

	baselineExec, baselineExecErr := ft.Execute(context.Background(), "federation/list-ephemeral", statusOnlyArgs)
	if baselineExecErr != nil {
		t.Fatalf("baseline Execute with status filter should succeed, got error: %v", baselineExecErr)
	}
	baselineDirect, baselineDirectErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", statusOnlyArgs)
	if baselineDirectErr != nil {
		t.Fatalf("baseline ExecuteTool with status filter should succeed, got error: %v", baselineDirectErr)
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", blankSwarmArgs)
	if execErr != nil {
		t.Fatalf("Execute should treat blank swarmId as no-op when status filter is present, got error: %v", execErr)
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", blankSwarmArgs)
	if directErr != nil {
		t.Fatalf("ExecuteTool should treat blank swarmId as no-op when status filter is present, got error: %v", directErr)
	}

	extractIDs := func(result shared.MCPToolResult) []string {
		t.Helper()
		agents, ok := result.Data.([]*shared.EphemeralAgent)
		if !ok {
			t.Fatalf("expected result data type []*shared.EphemeralAgent, got %T", result.Data)
		}
		ids := make([]string, 0, len(agents))
		for _, agent := range agents {
			if agent == nil {
				continue
			}
			if agent.Status != shared.EphemeralStatusTerminated {
				t.Fatalf("expected only terminated agents, got %q", agent.Status)
			}
			ids = append(ids, agent.ID)
		}
		return ids
	}

	baselineExecIDs := extractIDs(*baselineExec)
	baselineDirectIDs := extractIDs(baselineDirect)
	execIDs := extractIDs(*execResult)
	directIDs := extractIDs(directResult)

	if !reflect.DeepEqual(execIDs, baselineExecIDs) {
		t.Fatalf("expected Execute blank-swarm IDs %v to match status-only baseline IDs %v", execIDs, baselineExecIDs)
	}
	if !reflect.DeepEqual(directIDs, baselineDirectIDs) {
		t.Fatalf("expected ExecuteTool blank-swarm IDs %v to match status-only baseline IDs %v", directIDs, baselineDirectIDs)
	}
	if !reflect.DeepEqual(execIDs, directIDs) {
		t.Fatalf("expected Execute and ExecuteTool blank-swarm ID parity, got Execute=%v ExecuteTool=%v", execIDs, directIDs)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ListEphemeralInvalidStatusParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"status": "unknown-status",
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/list-ephemeral", args)
	if execErr == nil {
		t.Fatal("expected Execute to return validation error for invalid status")
	}
	if execResult == nil {
		t.Fatal("expected Execute result for invalid status")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/list-ephemeral", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool to return validation error for invalid status")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execArgs := map[string]interface{}{
		"swarmId":   "swarm-execute",
		"name":      "Execute Swarm",
		"maxAgents": float64(4),
	}
	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", execArgs)
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directArgs := map[string]interface{}{
		"swarmId":   "swarm-direct",
		"name":      "Direct Swarm",
		"maxAgents": float64(5),
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", directArgs)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute data map, got %T", execResult.Data)
	}
	if execData["registered"] != "swarm-execute" {
		t.Fatalf("expected Execute registered swarm to be swarm-execute, got %v", execData["registered"])
	}

	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool data map, got %T", directResult.Data)
	}
	if directData["registered"] != "swarm-direct" {
		t.Fatalf("expected ExecuteTool registered swarm to be swarm-direct, got %v", directData["registered"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmIntegerMaxAgentsParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":   "swarm-int",
		"name":      "Integer Swarm",
		"maxAgents": 7, // int
	})
	if execErr != nil {
		t.Fatalf("Execute should accept int maxAgents, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":   "swarm-int64",
		"name":      "Integer64 Swarm",
		"maxAgents": int64(9), // int64
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should accept int64 maxAgents, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool data map, got %T", directResult.Data)
	}

	if execData["registered"] != "swarm-int" {
		t.Fatalf("expected Execute registered swarm-int, got %v", execData["registered"])
	}
	if directData["registered"] != "swarm-int64" {
		t.Fatalf("expected ExecuteTool registered swarm-int64, got %v", directData["registered"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmInt64OutOfRangeParity(t *testing.T) {
	if strconv.IntSize >= 64 {
		t.Skip("int64 range matches int range on 64-bit platforms")
	}

	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"swarmId":   "swarm-overflow-int64",
		"name":      "Overflow Int64 Swarm",
		"maxAgents": int64(1 << 40),
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", args)
	if execErr == nil {
		t.Fatal("expected Execute validation error for out-of-range int64 maxAgents")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}
	if execResult.Error != "maxAgents is out of range" {
		t.Fatalf("expected Execute error %q, got %q", "maxAgents is out of range", execResult.Error)
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool validation error for out-of-range int64 maxAgents")
	}
	if directResult.Error != "maxAgents is out of range" {
		t.Fatalf("expected ExecuteTool error %q, got %q", "maxAgents is out of range", directResult.Error)
	}

	if execResult.Error != directResult.Error {
		t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmStringSliceCapabilitiesParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execCaps := []string{"go", " docker ", "", "   "}
	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":      "swarm-cap-exec",
		"name":         "Capabilities Execute",
		"maxAgents":    6,
		"capabilities": execCaps, // []string
	})
	if execErr != nil {
		t.Fatalf("Execute should accept []string capabilities, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directCaps := []string{"security", " ", ""}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":      "swarm-cap-direct",
		"name":         "Capabilities Direct",
		"maxAgents":    int64(3),
		"capabilities": directCaps, // []string
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should accept []string capabilities, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSwarm, ok := hub.GetSwarm("swarm-cap-exec")
	if !ok {
		t.Fatal("expected swarm-cap-exec to be registered")
	}
	expectedExecCaps := []string{"go", "docker"}
	if len(execSwarm.Capabilities) != len(expectedExecCaps) {
		t.Fatalf("expected Execute capabilities length %d, got %d", len(expectedExecCaps), len(execSwarm.Capabilities))
	}
	for _, cap := range expectedExecCaps {
		found := false
		for _, existing := range execSwarm.Capabilities {
			if existing == cap {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Execute swarm capabilities to contain %q, got %v", cap, execSwarm.Capabilities)
		}
	}

	directSwarm, ok := hub.GetSwarm("swarm-cap-direct")
	if !ok {
		t.Fatal("expected swarm-cap-direct to be registered")
	}
	expectedDirectCaps := []string{"security"}
	if len(directSwarm.Capabilities) != len(expectedDirectCaps) {
		t.Fatalf("expected ExecuteTool capabilities length %d, got %d", len(expectedDirectCaps), len(directSwarm.Capabilities))
	}
	if len(directSwarm.Capabilities) > 0 && directSwarm.Capabilities[0] != expectedDirectCaps[0] {
		t.Fatalf("expected ExecuteTool capability %q, got %v", expectedDirectCaps[0], directSwarm.Capabilities)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmInterfaceSliceCapabilitiesParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":      "swarm-cap-iface-exec",
		"name":         "Interface Caps Execute",
		"maxAgents":    float64(4),
		"capabilities": []interface{}{"go", " ", "docker", "go"},
	})
	if execErr != nil {
		t.Fatalf("Execute should accept []interface{} capabilities, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":      "swarm-cap-iface-direct",
		"name":         "Interface Caps Direct",
		"maxAgents":    float64(3),
		"capabilities": []interface{}{"security", "", "security"},
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should accept []interface{} capabilities, got error: %v", directErr)
	}
	if !execResult.Success || !directResult.Success {
		t.Fatalf("expected successful results, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSwarm, ok := hub.GetSwarm("swarm-cap-iface-exec")
	if !ok {
		t.Fatal("expected swarm-cap-iface-exec to be registered")
	}
	expectedExecCaps := []string{"go", "docker"}
	if len(execSwarm.Capabilities) != len(expectedExecCaps) {
		t.Fatalf("expected Execute capabilities length %d, got %d", len(expectedExecCaps), len(execSwarm.Capabilities))
	}
	for _, cap := range expectedExecCaps {
		found := false
		for _, existing := range execSwarm.Capabilities {
			if existing == cap {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected Execute swarm capabilities to contain %q, got %v", cap, execSwarm.Capabilities)
		}
	}

	directSwarm, ok := hub.GetSwarm("swarm-cap-iface-direct")
	if !ok {
		t.Fatal("expected swarm-cap-iface-direct to be registered")
	}
	expectedDirectCaps := []string{"security"}
	if len(directSwarm.Capabilities) != len(expectedDirectCaps) {
		t.Fatalf("expected ExecuteTool capabilities length %d, got %d", len(expectedDirectCaps), len(directSwarm.Capabilities))
	}
	if directSwarm.Capabilities[0] != expectedDirectCaps[0] {
		t.Fatalf("expected ExecuteTool capability %q, got %v", expectedDirectCaps[0], directSwarm.Capabilities)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmTrimsWhitespaceInputsParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":   "  swarm-trim-exec  ",
		"name":      "  Execute Swarm  ",
		"endpoint":  "  http://execute.local  ",
		"maxAgents": float64(4),
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed with padded fields, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", map[string]interface{}{
		"swarmId":   "  swarm-trim-direct  ",
		"name":      "  Direct Swarm  ",
		"endpoint":  "  http://direct.local  ",
		"maxAgents": int64(5),
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with padded fields, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSwarm, ok := hub.GetSwarm("swarm-trim-exec")
	if !ok {
		t.Fatal("expected trimmed swarm ID swarm-trim-exec to exist")
	}
	if execSwarm.Name != "Execute Swarm" {
		t.Fatalf("expected trimmed Execute swarm name, got %q", execSwarm.Name)
	}
	if execSwarm.Endpoint != "http://execute.local" {
		t.Fatalf("expected trimmed Execute endpoint, got %q", execSwarm.Endpoint)
	}

	directSwarm, ok := hub.GetSwarm("swarm-trim-direct")
	if !ok {
		t.Fatal("expected trimmed swarm ID swarm-trim-direct to exist")
	}
	if directSwarm.Name != "Direct Swarm" {
		t.Fatalf("expected trimmed ExecuteTool swarm name, got %q", directSwarm.Name)
	}
	if directSwarm.Endpoint != "http://direct.local" {
		t.Fatalf("expected trimmed ExecuteTool endpoint, got %q", directSwarm.Endpoint)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	for _, swarmID := range []string{"swarm-exec", "swarm-direct"} {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-exec",
		"type":    "coder",
		"task":    "implement feature A",
		"ttl":     float64(60000),
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-direct",
		"type":    "reviewer",
		"task":    "review feature A",
		"ttl":     float64(45000),
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	if execSpawn.Status != directSpawn.Status {
		t.Fatalf("expected spawn status parity, got Execute=%s ExecuteTool=%s", execSpawn.Status, directSpawn.Status)
	}
	if execSpawn.SwarmID != "swarm-exec" || directSpawn.SwarmID != "swarm-direct" {
		t.Fatalf("expected spawned swarm IDs to match requests, got Execute=%s ExecuteTool=%s", execSpawn.SwarmID, directSpawn.SwarmID)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralIntegerTTLParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	for _, swarmID := range []string{"swarm-ttl-exec", "swarm-ttl-direct"} {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-ttl-exec",
		"type":    "coder",
		"task":    "implement ttl-int case",
		"ttl":     12345, // int
	})
	if execErr != nil {
		t.Fatalf("Execute should accept int ttl, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-ttl-direct",
		"type":    "reviewer",
		"task":    "implement ttl-int64 case",
		"ttl":     int64(23456), // int64
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should accept int64 ttl, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	if execSpawn.EstimatedTTL != 12345 {
		t.Fatalf("expected Execute estimated ttl 12345, got %d", execSpawn.EstimatedTTL)
	}
	if directSpawn.EstimatedTTL != 23456 {
		t.Fatalf("expected ExecuteTool estimated ttl 23456, got %d", directSpawn.EstimatedTTL)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralDefaultTTLParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	for _, swarmID := range []string{"swarm-default-ttl-exec", "swarm-default-ttl-direct"} {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)
	expectedTTL := shared.DefaultFederationConfig().DefaultAgentTTL

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-default-ttl-exec",
		"type":    "coder",
		"task":    "execute default ttl case",
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed without ttl, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "swarm-default-ttl-direct",
		"type":    "reviewer",
		"task":    "direct default ttl case",
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed without ttl, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	if execSpawn.EstimatedTTL != expectedTTL {
		t.Fatalf("expected Execute estimated ttl %d, got %d", expectedTTL, execSpawn.EstimatedTTL)
	}
	if directSpawn.EstimatedTTL != expectedTTL {
		t.Fatalf("expected ExecuteTool estimated ttl %d, got %d", expectedTTL, directSpawn.EstimatedTTL)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralTrimsWhitespaceInputsParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-trim-spawn",
		Name:      "Trim Spawn Swarm",
		MaxAgents: 5,
	}); err != nil {
		t.Fatalf("failed to register trim spawn swarm: %v", err)
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "  swarm-trim-spawn  ",
		"type":    "  coder  ",
		"task":    "  implement trimmed task  ",
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed with padded fields, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", map[string]interface{}{
		"swarmId": "  swarm-trim-spawn  ",
		"type":    "  reviewer  ",
		"task":    "  review trimmed task  ",
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with padded fields, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	execAgent, ok := hub.GetAgent(execSpawn.AgentID)
	if !ok {
		t.Fatalf("expected Execute agent %s to exist", execSpawn.AgentID)
	}
	if execAgent.SwarmID != "swarm-trim-spawn" || execAgent.Type != "coder" || execAgent.Task != "implement trimmed task" {
		t.Fatalf("expected trimmed Execute agent fields, got swarm=%q type=%q task=%q", execAgent.SwarmID, execAgent.Type, execAgent.Task)
	}

	directAgent, ok := hub.GetAgent(directSpawn.AgentID)
	if !ok {
		t.Fatalf("expected ExecuteTool agent %s to exist", directSpawn.AgentID)
	}
	if directAgent.SwarmID != "swarm-trim-spawn" || directAgent.Type != "reviewer" || directAgent.Task != "review trimmed task" {
		t.Fatalf("expected trimmed ExecuteTool agent fields, got swarm=%q type=%q task=%q", directAgent.SwarmID, directAgent.Type, directAgent.Task)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralStringSliceCapabilitiesParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	// Available swarm without required capability.
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:       "swarm-general",
		Name:          "General Swarm",
		MaxAgents:     5,
		Capabilities:  []string{"general"},
		CurrentAgents: 0,
	}); err != nil {
		t.Fatalf("failed to register general swarm: %v", err)
	}

	// Matching capability swarm but intentionally at capacity (CurrentAgents >= MaxAgents).
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:       "swarm-gpu-full",
		Name:          "GPU Full Swarm",
		MaxAgents:     1,
		CurrentAgents: 1,
		Capabilities:  []string{"gpu"},
	}); err != nil {
		t.Fatalf("failed to register gpu-full swarm: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"type":         "coder",
		"task":         "use gpu acceleration",
		"capabilities": []string{"gpu", " ", ""}, // []string path with blanks
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", args)
	if execErr == nil {
		t.Fatal("expected Execute error when required capability has no available swarm")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool error when required capability has no available swarm")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralStringSliceCapabilitiesSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-gpu-ready",
		Name:         "GPU Ready Swarm",
		MaxAgents:    5,
		Capabilities: []string{"gpu"},
	}); err != nil {
		t.Fatalf("failed to register gpu-ready swarm: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"type":         "coder",
		"task":         "run gpu workload",
		"capabilities": []string{"gpu", " ", ""}, // []string path with blanks
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed with cleaned capabilities, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with cleaned capabilities, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	if execSpawn.SwarmID != "swarm-gpu-ready" || directSpawn.SwarmID != "swarm-gpu-ready" {
		t.Fatalf("expected both spawns to target swarm-gpu-ready, got Execute=%s ExecuteTool=%s", execSpawn.SwarmID, directSpawn.SwarmID)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_SpawnEphemeralInterfaceSliceCapabilitiesSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:      "swarm-ml-ready",
		Name:         "ML Ready Swarm",
		MaxAgents:    5,
		Capabilities: []string{"ml"},
	}); err != nil {
		t.Fatalf("failed to register ml-ready swarm: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"type":         "analyst",
		"task":         "run model scoring",
		"capabilities": []interface{}{"ml", " ", ""},
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/spawn-ephemeral", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed with cleaned []interface{} capabilities, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/spawn-ephemeral", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed with cleaned []interface{} capabilities, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execSpawn, ok := execResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected Execute data type *shared.SpawnResult, got %T", execResult.Data)
	}
	directSpawn, ok := directResult.Data.(*shared.SpawnResult)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.SpawnResult, got %T", directResult.Data)
	}

	if execSpawn.SwarmID != "swarm-ml-ready" || directSpawn.SwarmID != "swarm-ml-ready" {
		t.Fatalf("expected both spawns to target swarm-ml-ready, got Execute=%s ExecuteTool=%s", execSpawn.SwarmID, directSpawn.SwarmID)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_TerminateEphemeralSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	for _, swarmID := range []string{"swarm-exec", "swarm-direct"} {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	execSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-exec",
		Type:    "coder",
		Task:    "execute-terminate-path",
	})
	if err != nil {
		t.Fatalf("failed to spawn Execute test agent: %v", err)
	}
	directSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-direct",
		Type:    "reviewer",
		Task:    "direct-terminate-path",
	})
	if err != nil {
		t.Fatalf("failed to spawn ExecuteTool test agent: %v", err)
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{
		"agentId": execSpawn.AgentID,
		"error":   "intentional-terminate",
	})
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{
		"agentId": directSpawn.AgentID,
		"error":   "intentional-terminate-direct",
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute terminate data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool terminate data map, got %T", directResult.Data)
	}

	if execData["terminated"] != execSpawn.AgentID {
		t.Fatalf("expected Execute to terminate %s, got %v", execSpawn.AgentID, execData["terminated"])
	}
	if directData["terminated"] != directSpawn.AgentID {
		t.Fatalf("expected ExecuteTool to terminate %s, got %v", directSpawn.AgentID, directData["terminated"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_TerminateEphemeralTrimmedAgentIDParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-terminate-trim",
		Name:      "Terminate Trim Swarm",
		MaxAgents: 5,
	}); err != nil {
		t.Fatalf("failed to register terminate trim swarm: %v", err)
	}

	execSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-terminate-trim",
		Type:    "coder",
		Task:    "exec terminate trim",
	})
	if err != nil {
		t.Fatalf("failed to spawn Execute trim agent: %v", err)
	}
	directSpawn, err := hub.SpawnEphemeralAgent(shared.SpawnEphemeralOptions{
		SwarmID: "swarm-terminate-trim",
		Type:    "reviewer",
		Task:    "direct terminate trim",
	})
	if err != nil {
		t.Fatalf("failed to spawn ExecuteTool trim agent: %v", err)
	}

	ft := NewFederationTools(hub)

	execResult, execErr := ft.Execute(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{
		"agentId": "  " + execSpawn.AgentID + "  ",
	})
	if execErr != nil {
		t.Fatalf("Execute should accept trimmed agent ID, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute terminate result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{
		"agentId": "  " + directSpawn.AgentID + "  ",
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool should accept trimmed agent ID, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute terminate data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool terminate data map, got %T", directResult.Data)
	}

	if execData["terminated"] != execSpawn.AgentID {
		t.Fatalf("expected Execute terminated agent %s, got %v", execSpawn.AgentID, execData["terminated"])
	}
	if directData["terminated"] != directSpawn.AgentID {
		t.Fatalf("expected ExecuteTool terminated agent %s, got %v", directSpawn.AgentID, directData["terminated"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_BroadcastSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-source",
		Name:      "Source Swarm",
		MaxAgents: 10,
	}); err != nil {
		t.Fatalf("failed to register source swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-target",
		Name:      "Target Swarm",
		MaxAgents: 10,
	}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"sourceSwarmId": "swarm-source",
		"payload": map[string]interface{}{
			"event": "deployment",
		},
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/broadcast", args)
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/broadcast", args)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execMsg, ok := execResult.Data.(*shared.FederationMessage)
	if !ok {
		t.Fatalf("expected Execute data type *shared.FederationMessage, got %T", execResult.Data)
	}
	directMsg, ok := directResult.Data.(*shared.FederationMessage)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.FederationMessage, got %T", directResult.Data)
	}

	if execMsg.Type != directMsg.Type {
		t.Fatalf("expected message type parity, got Execute=%v ExecuteTool=%v", execMsg.Type, directMsg.Type)
	}
	if execMsg.SourceSwarmID != "swarm-source" || directMsg.SourceSwarmID != "swarm-source" {
		t.Fatalf("expected both messages to use source swarm-source, got Execute=%s ExecuteTool=%s", execMsg.SourceSwarmID, directMsg.SourceSwarmID)
	}
}

func TestFederationTools_BroadcastReturnsDefensiveMessageCopies(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-broadcast-source",
		Name:      "Broadcast Source",
		MaxAgents: 10,
	}); err != nil {
		t.Fatalf("failed to register source swarm: %v", err)
	}
	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-broadcast-target",
		Name:      "Broadcast Target",
		MaxAgents: 10,
	}); err != nil {
		t.Fatalf("failed to register target swarm: %v", err)
	}

	ft := NewFederationTools(hub)

	execPayload := map[string]interface{}{
		"event":  "execute-original",
		"meta":   map[string]interface{}{"version": "1.0.0"},
		"labels": map[string]string{"env": "prod"},
	}
	execResult, execErr := ft.Execute(context.Background(), "federation/broadcast", map[string]interface{}{
		"sourceSwarmId": "swarm-broadcast-source",
		"payload":       execPayload,
	})
	if execErr != nil {
		t.Fatalf("Execute broadcast should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute broadcast result")
	}
	execMsg, ok := execResult.Data.(*shared.FederationMessage)
	if !ok {
		t.Fatalf("expected Execute data type *shared.FederationMessage, got %T", execResult.Data)
	}

	// Mutate original input payload and returned payload copy.
	execPayload["event"] = "execute-mutated-input"
	execPayload["meta"].(map[string]interface{})["version"] = "9.9.9"
	execPayload["labels"].(map[string]string)["env"] = "staging"
	execResultPayload, ok := execMsg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute message payload map, got %T", execMsg.Payload)
	}
	execResultPayload["event"] = "execute-mutated-output"
	execResultPayload["meta"].(map[string]interface{})["version"] = "8.8.8"
	execResultPayload["labels"].(map[string]string)["env"] = "qa"

	directPayload := map[string]interface{}{
		"event":  "direct-original",
		"meta":   map[string]interface{}{"version": "2.0.0"},
		"labels": map[string]string{"env": "dev"},
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/broadcast", map[string]interface{}{
		"sourceSwarmId": "swarm-broadcast-source",
		"payload":       directPayload,
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool broadcast should succeed, got error: %v", directErr)
	}
	directMsg, ok := directResult.Data.(*shared.FederationMessage)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.FederationMessage, got %T", directResult.Data)
	}

	directPayload["event"] = "direct-mutated-input"
	directPayload["meta"].(map[string]interface{})["version"] = "7.7.7"
	directPayload["labels"].(map[string]string)["env"] = "local"
	directResultPayload, ok := directMsg.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool message payload map, got %T", directMsg.Payload)
	}
	directResultPayload["event"] = "direct-mutated-output"
	directResultPayload["meta"].(map[string]interface{})["version"] = "6.6.6"
	directResultPayload["labels"].(map[string]string)["env"] = "ci"

	storedExec, ok := hub.GetMessage(execMsg.ID)
	if !ok {
		t.Fatalf("expected stored Execute message %s", execMsg.ID)
	}
	storedExecPayload, ok := storedExec.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored Execute payload map, got %T", storedExec.Payload)
	}
	if storedExecPayload["event"] != "execute-original" {
		t.Fatalf("expected stored Execute payload event to remain original, got %v", storedExecPayload["event"])
	}
	storedExecMeta, ok := storedExecPayload["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored Execute payload meta map, got %T", storedExecPayload["meta"])
	}
	if storedExecMeta["version"] != "1.0.0" {
		t.Fatalf("expected stored Execute payload meta version to remain original, got %v", storedExecMeta["version"])
	}
	storedExecLabels, ok := storedExecPayload["labels"].(map[string]string)
	if !ok {
		t.Fatalf("expected stored Execute labels map[string]string, got %T", storedExecPayload["labels"])
	}
	if storedExecLabels["env"] != "prod" {
		t.Fatalf("expected stored Execute labels env to remain prod, got %v", storedExecLabels["env"])
	}

	storedDirect, ok := hub.GetMessage(directMsg.ID)
	if !ok {
		t.Fatalf("expected stored ExecuteTool message %s", directMsg.ID)
	}
	storedDirectPayload, ok := storedDirect.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored ExecuteTool payload map, got %T", storedDirect.Payload)
	}
	if storedDirectPayload["event"] != "direct-original" {
		t.Fatalf("expected stored ExecuteTool payload event to remain original, got %v", storedDirectPayload["event"])
	}
	storedDirectMeta, ok := storedDirectPayload["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored ExecuteTool payload meta map, got %T", storedDirectPayload["meta"])
	}
	if storedDirectMeta["version"] != "2.0.0" {
		t.Fatalf("expected stored ExecuteTool payload meta version to remain original, got %v", storedDirectMeta["version"])
	}
	storedDirectLabels, ok := storedDirectPayload["labels"].(map[string]string)
	if !ok {
		t.Fatalf("expected stored ExecuteTool labels map[string]string, got %T", storedDirectPayload["labels"])
	}
	if storedDirectLabels["env"] != "dev" {
		t.Fatalf("expected stored ExecuteTool labels env to remain dev, got %v", storedDirectLabels["env"])
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ProposeSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registered := []string{"swarm-proposer-exec", "swarm-proposer-direct", "swarm-voter-a", "swarm-voter-b"}
	for _, swarmID := range registered {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)

	execArgs := map[string]interface{}{
		"proposerId":   "swarm-proposer-exec",
		"proposalType": "scale-up",
		"value": map[string]interface{}{
			"maxAgents": 20,
		},
	}
	execResult, execErr := ft.Execute(context.Background(), "federation/propose", execArgs)
	if execErr != nil {
		t.Fatalf("Execute should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute result to be non-nil")
	}

	directArgs := map[string]interface{}{
		"proposerId":   "swarm-proposer-direct",
		"proposalType": "scale-down",
		"value": map[string]interface{}{
			"maxAgents": 5,
		},
	}
	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/propose", directArgs)
	if directErr != nil {
		t.Fatalf("ExecuteTool should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execProposal, ok := execResult.Data.(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected Execute data type *shared.FederationProposal, got %T", execResult.Data)
	}
	directProposal, ok := directResult.Data.(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected ExecuteTool data type *shared.FederationProposal, got %T", directResult.Data)
	}

	if execProposal.ProposerID != "swarm-proposer-exec" {
		t.Fatalf("expected Execute proposer swarm-proposer-exec, got %s", execProposal.ProposerID)
	}
	if directProposal.ProposerID != "swarm-proposer-direct" {
		t.Fatalf("expected ExecuteTool proposer swarm-proposer-direct, got %s", directProposal.ProposerID)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_VoteSuccessParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	registered := []string{"swarm-proposer-exec", "swarm-proposer-direct", "swarm-voter-exec", "swarm-voter-direct"}
	for _, swarmID := range registered {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)

	execProposalRaw, execProposalErr := ft.ExecuteTool(context.Background(), "federation/propose", map[string]interface{}{
		"proposerId":   "swarm-proposer-exec",
		"proposalType": "exec-vote",
		"value":        map[string]interface{}{"threshold": 1},
	})
	if execProposalErr != nil {
		t.Fatalf("failed to create proposal for Execute vote path: %v", execProposalErr)
	}
	execProposal, ok := execProposalRaw.Data.(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected exec proposal type *shared.FederationProposal, got %T", execProposalRaw.Data)
	}

	directProposalRaw, directProposalErr := ft.ExecuteTool(context.Background(), "federation/propose", map[string]interface{}{
		"proposerId":   "swarm-proposer-direct",
		"proposalType": "direct-vote",
		"value":        map[string]interface{}{"threshold": 2},
	})
	if directProposalErr != nil {
		t.Fatalf("failed to create proposal for ExecuteTool vote path: %v", directProposalErr)
	}
	directProposal, ok := directProposalRaw.Data.(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected direct proposal type *shared.FederationProposal, got %T", directProposalRaw.Data)
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/vote", map[string]interface{}{
		"voterId":    "swarm-voter-exec",
		"proposalId": execProposal.ID,
		"approve":    true,
	})
	if execErr != nil {
		t.Fatalf("Execute vote should succeed, got error: %v", execErr)
	}
	if execResult == nil {
		t.Fatal("expected Execute vote result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/vote", map[string]interface{}{
		"voterId":    "swarm-voter-direct",
		"proposalId": directProposal.ID,
		"approve":    true,
	})
	if directErr != nil {
		t.Fatalf("ExecuteTool vote should succeed, got error: %v", directErr)
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	execData, ok := execResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Execute vote data map, got %T", execResult.Data)
	}
	directData, ok := directResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected ExecuteTool vote data map, got %T", directResult.Data)
	}

	if execData["voted"] != true || directData["voted"] != true {
		t.Fatalf("expected voted=true for both paths, got Execute=%v ExecuteTool=%v", execData["voted"], directData["voted"])
	}
}

func TestFederationTools_ProposeAndVoteReturnDefensiveProposalCopies(t *testing.T) {
	config := shared.DefaultFederationConfig()
	config.ConsensusQuorum = 1.0
	hub := federation.NewFederationHub(config)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	for _, swarmID := range []string{"swarm-propose-source", "swarm-propose-voter"} {
		if err := hub.RegisterSwarm(shared.SwarmRegistration{
			SwarmID:   swarmID,
			Name:      swarmID,
			MaxAgents: 10,
		}); err != nil {
			t.Fatalf("failed to register swarm %s: %v", swarmID, err)
		}
	}

	ft := NewFederationTools(hub)

	value := map[string]interface{}{
		"policy": map[string]interface{}{"mode": "balanced"},
		"tags":   map[string]string{"tier": "gold"},
	}
	proposeResult, proposeErr := ft.Execute(context.Background(), "federation/propose", map[string]interface{}{
		"proposerId":   "swarm-propose-source",
		"proposalType": "policy-update",
		"value":        value,
	})
	if proposeErr != nil {
		t.Fatalf("Execute propose should succeed, got error: %v", proposeErr)
	}
	if proposeResult == nil {
		t.Fatal("expected propose result")
	}
	proposalCopy, ok := proposeResult.Data.(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected propose data type *shared.FederationProposal, got %T", proposeResult.Data)
	}

	// Mutate original input and returned proposal copy.
	value["policy"].(map[string]interface{})["mode"] = "mutated-input"
	value["newField"] = "mutated-input"
	value["tags"].(map[string]string)["tier"] = "silver"
	copyValue, ok := proposalCopy.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected proposal copy value map, got %T", proposalCopy.Value)
	}
	copyValue["policy"].(map[string]interface{})["mode"] = "mutated-output"
	copyValue["newField"] = "mutated-output"
	copyValue["tags"].(map[string]string)["tier"] = "platinum"
	proposalCopy.Votes["external-mutated"] = true

	storedProposal, ok := hub.GetProposal(proposalCopy.ID)
	if !ok {
		t.Fatalf("expected stored proposal %s", proposalCopy.ID)
	}
	storedValue, ok := storedProposal.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored proposal value map, got %T", storedProposal.Value)
	}
	storedPolicy, ok := storedValue["policy"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored proposal policy map, got %T", storedValue["policy"])
	}
	if storedPolicy["mode"] != "balanced" {
		t.Fatalf("expected stored policy mode to remain balanced, got %v", storedPolicy["mode"])
	}
	if _, exists := storedValue["newField"]; exists {
		t.Fatalf("expected stored proposal value to not include newField, got %v", storedValue["newField"])
	}
	storedTags, ok := storedValue["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected stored proposal tags map[string]string, got %T", storedValue["tags"])
	}
	if storedTags["tier"] != "gold" {
		t.Fatalf("expected stored proposal tags tier to remain gold, got %v", storedTags["tier"])
	}
	if _, exists := storedProposal.Votes["external-mutated"]; exists {
		t.Fatal("expected stored proposal votes to ignore external mutation")
	}

	voteResult, voteErr := ft.ExecuteTool(context.Background(), "federation/vote", map[string]interface{}{
		"voterId":    "swarm-propose-voter",
		"proposalId": proposalCopy.ID,
		"approve":    true,
	})
	if voteErr != nil {
		t.Fatalf("ExecuteTool vote should succeed, got error: %v", voteErr)
	}
	voteData, ok := voteResult.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected vote data map, got %T", voteResult.Data)
	}
	voteProposalCopy, ok := voteData["proposal"].(*shared.FederationProposal)
	if !ok {
		t.Fatalf("expected vote proposal copy type *shared.FederationProposal, got %T", voteData["proposal"])
	}

	voteProposalCopy.Votes["another-external-mutated"] = true
	voteProposalValue, ok := voteProposalCopy.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected vote proposal value map, got %T", voteProposalCopy.Value)
	}
	voteProposalValue["policy"].(map[string]interface{})["mode"] = "mutated-after-vote"
	voteProposalValue["tags"].(map[string]string)["tier"] = "diamond"

	storedAfterVote, ok := hub.GetProposal(proposalCopy.ID)
	if !ok {
		t.Fatalf("expected stored proposal %s after vote", proposalCopy.ID)
	}
	if _, exists := storedAfterVote.Votes["another-external-mutated"]; exists {
		t.Fatal("expected stored proposal votes to ignore vote-result mutation")
	}
	storedAfterVoteValue, ok := storedAfterVote.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored proposal value map after vote, got %T", storedAfterVote.Value)
	}
	storedAfterVotePolicy, ok := storedAfterVoteValue["policy"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected stored proposal policy map after vote, got %T", storedAfterVoteValue["policy"])
	}
	if storedAfterVotePolicy["mode"] != "balanced" {
		t.Fatalf("expected stored policy mode to remain balanced after vote copy mutation, got %v", storedAfterVotePolicy["mode"])
	}
	storedAfterVoteTags, ok := storedAfterVoteValue["tags"].(map[string]string)
	if !ok {
		t.Fatalf("expected stored proposal tags map[string]string after vote, got %T", storedAfterVoteValue["tags"])
	}
	if storedAfterVoteTags["tier"] != "gold" {
		t.Fatalf("expected stored proposal tags tier to remain gold after vote mutation, got %v", storedAfterVoteTags["tier"])
	}
}

func TestFederationTools_Execute_ValidationErrorPropagation(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	result, err := ft.Execute(context.Background(), "federation/terminate-ephemeral", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected validation error for missing agentId")
	}
	if result == nil {
		t.Fatal("expected non-nil result for validation failure")
	}
	if result.Success {
		t.Fatal("validation failure should not report success")
	}
	if result.Error == "" {
		t.Fatal("expected validation error message in result")
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ValidationErrorParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{} // missing required agentId

	execResult, execErr := ft.Execute(context.Background(), "federation/terminate-ephemeral", args)
	if execErr == nil {
		t.Fatal("expected Execute validation error")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/terminate-ephemeral", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool validation error")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}

	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RegisterSwarmValidationParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"name":      "missing-id",
		"maxAgents": float64(5),
	} // swarmId intentionally omitted

	execResult, execErr := ft.Execute(context.Background(), "federation/register-swarm", args)
	if execErr == nil {
		t.Fatal("expected Execute validation error")
	}
	if execResult == nil {
		t.Fatal("expected Execute result")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/register-swarm", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool validation error")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ValidationParityForRequiredFields(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
	}{
		{
			name:     "spawn missing type",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"task": "implement feature",
			},
		},
		{
			name:     "spawn missing task",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
			},
		},
		{
			name:     "spawn non-integer ttl",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  12.5,
			},
		},
		{
			name:     "spawn non-finite ttl",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  math.Inf(1),
			},
		},
		{
			name:     "spawn out-of-range ttl",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  1e20,
			},
		},
		{
			name:     "spawn non-positive ttl",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  float64(0),
			},
		},
		{
			name:     "spawn blank type",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "   ",
				"task": "implement feature",
			},
		},
		{
			name:     "spawn blank task",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "   ",
			},
		},
		{
			name:     "spawn non-string swarmId",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"swarmId": float64(1),
				"type":    "coder",
				"task":    "implement feature",
			},
		},
		{
			name:     "spawn non-string type",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": float64(1),
				"task": "implement feature",
			},
		},
		{
			name:     "spawn non-string task",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": true,
			},
		},
		{
			name:     "spawn non-integer ttl type",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  "1000",
			},
		},
		{
			name:     "spawn non-array capabilities",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type":         "coder",
				"task":         "implement feature",
				"capabilities": "go",
			},
		},
		{
			name:     "spawn capabilities with non-string entry",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type":         "coder",
				"task":         "implement feature",
				"capabilities": []interface{}{"go", 1},
			},
		},
		{
			name:     "spawn nil capabilities",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type":         "coder",
				"task":         "implement feature",
				"capabilities": nil,
			},
		},
		{
			name:     "list non-string swarmId",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"swarmId": float64(1),
			},
		},
		{
			name:     "list non-string status",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"status": true,
			},
		},
		{
			name:     "list nil status",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"status": nil,
			},
		},
		{
			name:     "broadcast missing sourceSwarmId",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"payload": map[string]interface{}{"event": "x"},
			},
		},
		{
			name:     "broadcast missing payload",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "swarm-1",
			},
		},
		{
			name:     "broadcast non-object payload",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "swarm-1",
				"payload":       "not-an-object",
			},
		},
		{
			name:     "broadcast blank sourceSwarmId",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "   ",
				"payload":       map[string]interface{}{"event": "x"},
			},
		},
		{
			name:     "broadcast non-string sourceSwarmId",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": float64(1),
				"payload":       map[string]interface{}{"event": "x"},
			},
		},
		{
			name:     "propose missing proposerId",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposalType": "scaling",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose blank proposerId",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "   ",
				"proposalType": "scaling",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose non-string proposerId",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   float64(1),
				"proposalType": "scaling",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose missing proposalType",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId": "swarm-1",
				"value":      map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose blank proposalType",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "   ",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose non-string proposalType",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": true,
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "propose missing value",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "scaling",
			},
		},
		{
			name:     "propose non-object value",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "scaling",
				"value":        "not-an-object",
			},
		},
		{
			name:     "vote missing voterId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"proposalId": "p-1",
				"approve":    true,
			},
		},
		{
			name:     "vote blank voterId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "   ",
				"proposalId": "p-1",
				"approve":    true,
			},
		},
		{
			name:     "vote non-string voterId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    float64(1),
				"proposalId": "p-1",
				"approve":    true,
			},
		},
		{
			name:     "vote missing proposalId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId": "swarm-1",
				"approve": true,
			},
		},
		{
			name:     "vote blank proposalId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "   ",
				"approve":    true,
			},
		},
		{
			name:     "vote non-string proposalId",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": float64(1),
				"approve":    true,
			},
		},
		{
			name:     "vote missing approve flag",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "p-1",
			},
		},
		{
			name:     "vote non-boolean approve",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "p-1",
				"approve":    "true",
			},
		},
		{
			name:     "register swarm missing name",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"maxAgents": float64(5),
			},
		},
		{
			name:     "register swarm missing maxAgents",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId": "swarm-1",
				"name":    "swarm-one",
			},
		},
		{
			name:     "register swarm blank name",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "   ",
				"maxAgents": float64(5),
			},
		},
		{
			name:     "register swarm non-string swarmId",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   float64(1),
				"name":      "swarm-one",
				"maxAgents": float64(5),
			},
		},
		{
			name:     "register swarm non-string name",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      float64(1),
				"maxAgents": float64(5),
			},
		},
		{
			name:     "register swarm non-string endpoint",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"endpoint":  true,
				"maxAgents": float64(5),
			},
		},
		{
			name:     "register swarm non-integer maxAgents",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": 3.14,
			},
		},
		{
			name:     "register swarm non-finite maxAgents",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": math.Inf(1),
			},
		},
		{
			name:     "register swarm out-of-range maxAgents",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": 1e20,
			},
		},
		{
			name:     "register swarm non-integer maxAgents type",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": "5",
			},
		},
		{
			name:     "register swarm non-array capabilities",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":      "swarm-1",
				"name":         "swarm-one",
				"maxAgents":    float64(5),
				"capabilities": "go",
			},
		},
		{
			name:     "register swarm capabilities with non-string entry",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":      "swarm-1",
				"name":         "swarm-one",
				"maxAgents":    float64(5),
				"capabilities": []interface{}{"go", true},
			},
		},
		{
			name:     "register swarm nil capabilities",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":      "swarm-1",
				"name":         "swarm-one",
				"maxAgents":    float64(5),
				"capabilities": nil,
			},
		},
		{
			name:     "register swarm blank swarmId",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "   ",
				"name":      "swarm-one",
				"maxAgents": float64(5),
			},
		},
		{
			name:     "terminate blank agentId",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "   ",
			},
		},
		{
			name:     "terminate non-string agentId",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": float64(1),
			},
		},
		{
			name:     "terminate non-string error",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "agent-1",
				"error":   true,
			},
		},
		{
			name:     "terminate nil error",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "agent-1",
				"error":   nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, tc.args)
			if execErr == nil {
				t.Fatalf("expected Execute validation error for %s", tc.toolName)
			}
			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}

			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, tc.args)
			if directErr == nil {
				t.Fatalf("expected ExecuteTool validation error for %s", tc.toolName)
			}

			if execResult.Success != directResult.Success {
				t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
			}
			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
			}
		})
	}
}

func TestFederationTools_ExecuteAndExecuteTool_TypeValidationMessages(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name          string
		toolName      string
		args          map[string]interface{}
		expectedError string
	}{
		{
			name:     "spawn ttl wrong type",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  "1000",
			},
			expectedError: "ttl must be an integer",
		},
		{
			name:     "spawn ttl nil",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  nil,
			},
			expectedError: "ttl must be an integer",
		},
		{
			name:     "spawn ttl non-positive",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  float64(0),
			},
			expectedError: "ttl must be greater than 0",
		},
		{
			name:     "spawn ttl non-finite",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  math.Inf(1),
			},
			expectedError: "ttl must be a finite integer",
		},
		{
			name:     "spawn ttl out of range",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
				"ttl":  1e20,
			},
			expectedError: "ttl is out of range",
		},
		{
			name:     "spawn type nil",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": nil,
				"task": "implement feature",
			},
			expectedError: "type must be a string",
		},
		{
			name:     "spawn capabilities non-string entry",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type":         "coder",
				"task":         "implement feature",
				"capabilities": []interface{}{"go", 1},
			},
			expectedError: "capabilities must contain only strings",
		},
		{
			name:     "spawn capabilities nil",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type":         "coder",
				"task":         "implement feature",
				"capabilities": nil,
			},
			expectedError: "capabilities must be an array of strings",
		},
		{
			name:     "register endpoint wrong type",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"endpoint":  true,
				"maxAgents": float64(5),
			},
			expectedError: "endpoint must be a string",
		},
		{
			name:     "register maxAgents non-finite",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": math.Inf(1),
			},
			expectedError: "maxAgents must be a finite integer",
		},
		{
			name:     "register maxAgents out of range",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": 1e20,
			},
			expectedError: "maxAgents is out of range",
		},
		{
			name:     "register name nil",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      nil,
				"maxAgents": float64(5),
			},
			expectedError: "name must be a string",
		},
		{
			name:     "register capabilities nil",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":      "swarm-1",
				"name":         "swarm-one",
				"maxAgents":    float64(5),
				"capabilities": nil,
			},
			expectedError: "capabilities must be an array of strings",
		},
		{
			name:     "register maxAgents nil",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": nil,
			},
			expectedError: "maxAgents must be an integer",
		},
		{
			name:     "vote approve wrong type",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "proposal-1",
				"approve":    "true",
			},
			expectedError: "approve must be a boolean",
		},
		{
			name:     "vote approve nil",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "proposal-1",
				"approve":    nil,
			},
			expectedError: "approve must be a boolean",
		},
		{
			name:     "list status wrong type",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"status": true,
			},
			expectedError: "status must be a string",
		},
		{
			name:     "list swarmId nil",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"swarmId": nil,
			},
			expectedError: "swarmId must be a string",
		},
		{
			name:     "list status nil",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"status": nil,
			},
			expectedError: "status must be a string",
		},
		{
			name:     "list status invalid value",
			toolName: "federation/list-ephemeral",
			args: map[string]interface{}{
				"status": "invalid",
			},
			expectedError: "status must be one of: spawning, active, completing, terminated",
		},
		{
			name:     "terminate error wrong type",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "agent-1",
				"error":   true,
			},
			expectedError: "error must be a string",
		},
		{
			name:     "terminate error nil",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "agent-1",
				"error":   nil,
			},
			expectedError: "error must be a string",
		},
		{
			name:     "broadcast sourceSwarmId nil",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": nil,
				"payload":       map[string]interface{}{"event": "x"},
			},
			expectedError: "sourceSwarmId must be a string",
		},
		{
			name:     "broadcast payload nil",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "swarm-1",
				"payload":       nil,
			},
			expectedError: "payload is required",
		},
		{
			name:     "broadcast payload wrong type",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "swarm-1",
				"payload":       "event",
			},
			expectedError: "payload must be an object",
		},
		{
			name:     "propose value nil",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "scale",
				"value":        nil,
			},
			expectedError: "value is required",
		},
		{
			name:     "propose value wrong type",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "scale",
				"value":        "invalid",
			},
			expectedError: "value must be an object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, tc.args)
			if execErr == nil {
				t.Fatalf("expected Execute error for %s", tc.toolName)
			}
			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}
			if execResult.Error != tc.expectedError {
				t.Fatalf("expected Execute error %q, got %q", tc.expectedError, execResult.Error)
			}

			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, tc.args)
			if directErr == nil {
				t.Fatalf("expected ExecuteTool error for %s", tc.toolName)
			}
			if directResult.Error != tc.expectedError {
				t.Fatalf("expected ExecuteTool error %q, got %q", tc.expectedError, directResult.Error)
			}

			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
			}
		})
	}
}

func TestFederationTools_ExecuteAndExecuteTool_RuntimeErrorParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
	}{
		{
			name:     "spawn ephemeral without available swarm",
			toolName: "federation/spawn-ephemeral",
			args: map[string]interface{}{
				"type": "coder",
				"task": "implement feature",
			},
		},
		{
			name:     "terminate unknown agent",
			toolName: "federation/terminate-ephemeral",
			args: map[string]interface{}{
				"agentId": "missing-agent",
			},
		},
		{
			name:     "broadcast unknown source swarm",
			toolName: "federation/broadcast",
			args: map[string]interface{}{
				"sourceSwarmId": "missing-swarm",
				"payload":       map[string]interface{}{"event": "x"},
			},
		},
		{
			name:     "propose unknown proposer swarm",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "missing-swarm",
				"proposalType": "scale",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "vote unknown voter swarm",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "missing-voter",
				"proposalId": "missing-proposal",
				"approve":    true,
			},
		},
		{
			name:     "register swarm with non-positive capacity",
			toolName: "federation/register-swarm",
			args: map[string]interface{}{
				"swarmId":   "swarm-1",
				"name":      "swarm-one",
				"maxAgents": float64(0),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, tc.args)
			if execErr == nil {
				t.Fatalf("expected Execute runtime error for %s", tc.toolName)
			}
			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}

			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, tc.args)
			if directErr == nil {
				t.Fatalf("expected ExecuteTool runtime error for %s", tc.toolName)
			}

			if execResult.Success != directResult.Success {
				t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
			}
			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
			}
		})
	}
}

func TestFederationTools_ExecuteAndExecuteTool_ConsensusDisabledParity(t *testing.T) {
	config := shared.DefaultFederationConfig()
	config.EnableConsensus = false

	hub := federation.NewFederationHub(config)
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
	}{
		{
			name:     "propose when consensus disabled",
			toolName: "federation/propose",
			args: map[string]interface{}{
				"proposerId":   "swarm-1",
				"proposalType": "scale",
				"value":        map[string]interface{}{"maxAgents": 10},
			},
		},
		{
			name:     "vote when consensus disabled",
			toolName: "federation/vote",
			args: map[string]interface{}{
				"voterId":    "swarm-1",
				"proposalId": "proposal-1",
				"approve":    true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, tc.args)
			if execErr == nil {
				t.Fatalf("expected Execute error for %s", tc.toolName)
			}
			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}

			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, tc.args)
			if directErr == nil {
				t.Fatalf("expected ExecuteTool error for %s", tc.toolName)
			}

			if execResult.Success != directResult.Success {
				t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
			}
			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
			}
		})
	}
}

func TestFederationTools_ExecuteAndExecuteTool_VoteProposalNotFoundParity(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	if err := hub.RegisterSwarm(shared.SwarmRegistration{
		SwarmID:   "swarm-voter",
		Name:      "Swarm Voter",
		MaxAgents: 10,
	}); err != nil {
		t.Fatalf("failed to register voter swarm: %v", err)
	}

	ft := NewFederationTools(hub)
	args := map[string]interface{}{
		"voterId":    "swarm-voter",
		"proposalId": "missing-proposal",
		"approve":    true,
	}

	execResult, execErr := ft.Execute(context.Background(), "federation/vote", args)
	if execErr == nil {
		t.Fatal("expected Execute error for missing proposal")
	}
	if execResult == nil {
		t.Fatal("expected Execute result for missing proposal")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/vote", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool error for missing proposal")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_UnknownToolParity(t *testing.T) {
	ft := &FederationTools{}

	args := map[string]interface{}{}
	execResult, execErr := ft.Execute(context.Background(), "federation/unknown-tool", args)
	if execErr == nil {
		t.Fatal("expected Execute error for unknown tool")
	}
	if execResult == nil {
		t.Fatal("expected Execute result for unknown tool")
	}

	directResult, directErr := ft.ExecuteTool(context.Background(), "federation/unknown-tool", args)
	if directErr == nil {
		t.Fatal("expected ExecuteTool error for unknown tool")
	}

	if execResult.Success != directResult.Success {
		t.Fatalf("expected success parity, got Execute=%v ExecuteTool=%v", execResult.Success, directResult.Success)
	}
	if execResult.Error != directResult.Error {
		t.Fatalf("expected error message parity, got Execute=%q ExecuteTool=%q", execResult.Error, directResult.Error)
	}
}

func TestFederationTools_ExecuteAndExecuteTool_NilArgsSafety(t *testing.T) {
	hub := federation.NewFederationHubWithDefaults()
	if err := hub.Initialize(); err != nil {
		t.Fatalf("failed to initialize federation hub: %v", err)
	}
	t.Cleanup(func() {
		_ = hub.Shutdown()
	})

	ft := NewFederationTools(hub)

	tests := []struct {
		name          string
		toolName      string
		expectSuccess bool
		expectError   bool
	}{
		{name: "status with nil args", toolName: "federation/status", expectSuccess: true, expectError: false},
		{name: "list with nil args", toolName: "federation/list-ephemeral", expectSuccess: true, expectError: false},
		{name: "spawn with nil args", toolName: "federation/spawn-ephemeral", expectSuccess: false, expectError: true},
		{name: "terminate with nil args", toolName: "federation/terminate-ephemeral", expectSuccess: false, expectError: true},
		{name: "register with nil args", toolName: "federation/register-swarm", expectSuccess: false, expectError: true},
		{name: "broadcast with nil args", toolName: "federation/broadcast", expectSuccess: false, expectError: true},
		{name: "propose with nil args", toolName: "federation/propose", expectSuccess: false, expectError: true},
		{name: "vote with nil args", toolName: "federation/vote", expectSuccess: false, expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			execResult, execErr := ft.Execute(context.Background(), tc.toolName, nil)
			directResult, directErr := ft.ExecuteTool(context.Background(), tc.toolName, nil)

			if tc.expectError {
				if execErr == nil {
					t.Fatalf("expected Execute to return error for %s", tc.toolName)
				}
				if directErr == nil {
					t.Fatalf("expected ExecuteTool to return error for %s", tc.toolName)
				}
			} else {
				if execErr != nil {
					t.Fatalf("expected Execute success for %s, got error: %v", tc.toolName, execErr)
				}
				if directErr != nil {
					t.Fatalf("expected ExecuteTool success for %s, got error: %v", tc.toolName, directErr)
				}
			}

			if execResult == nil {
				t.Fatalf("expected Execute result for %s", tc.toolName)
			}
			if execResult.Success != tc.expectSuccess {
				t.Fatalf("expected Execute success=%v for %s, got %v", tc.expectSuccess, tc.toolName, execResult.Success)
			}

			if execResult.Success != directResult.Success {
				t.Fatalf("expected success parity for %s, got Execute=%v ExecuteTool=%v", tc.toolName, execResult.Success, directResult.Success)
			}
			if execResult.Error != directResult.Error {
				t.Fatalf("expected error parity for %s, got Execute=%q ExecuteTool=%q", tc.toolName, execResult.Error, directResult.Error)
			}
		})
	}
}
