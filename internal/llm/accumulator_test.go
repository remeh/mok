package llm

import (
	"testing"
)

func ptrInt(v int) *int {
	return &v
}

func TestAccumulateToolCall_NewCall(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	idx := ptrInt(0)
	delta := &PartialTC{
		Index:   idx,
		ID:      "call_1",
		Name:    "read_file",
		RawArgs: `{"file": "test.go"}`,
	}

	isNew, tc, tcOrder := AccumulateToolCall(tcMap, tcOrder, delta)
	if !isNew {
		t.Fatal("expected new tool call")
	}
	if tc.ID != "call_1" {
		t.Errorf("ID = %q, want 'call_1'", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("Name = %q, want 'read_file'", tc.Name)
	}
	if tc.RawArgs != `{"file": "test.go"}` {
		t.Errorf("RawArgs = %q", tc.RawArgs)
	}
	if len(tcOrder) != 1 {
		t.Errorf("order len = %d, want 1", len(tcOrder))
	}
}

func TestAccumulateToolCall_MergeByIndex(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	// First chunk: index + id + name
	idx0 := ptrInt(0)
	delta1 := &PartialTC{
		Index:   idx0,
		ID:      "call_1",
		Name:    "bash",
		RawArgs: `{"command":`,
	}
	isNew, _, tcOrder := AccumulateToolCall(tcMap, tcOrder, delta1)
	if !isNew {
		t.Fatal("expected new tool call for first chunk")
	}

	// Second chunk: index + args continuation
	delta2 := &PartialTC{
		Index:   idx0,
		RawArgs: `"ls -la"}`,
	}
	var tc *PartialTC
	isNew, tc, tcOrder = AccumulateToolCall(tcMap, tcOrder, delta2)
	if isNew {
		t.Fatal("expected merge, not new")
	}
	if tc.RawArgs != `{"command":"ls -la"}` {
		t.Errorf("RawArgs = %q, want %q", tc.RawArgs, `{"command":"ls -la"}`)
	}
	if len(tcOrder) != 1 {
		t.Errorf("order len = %d, want 1", len(tcOrder))
	}
}

func TestAccumulateToolCall_MergeByID(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	// First chunk with ID
	delta1 := &PartialTC{
		ID:      "call_1",
		Name:    "read_file",
		RawArgs: `{"file":`,
	}
	_, _, tcOrder = AccumulateToolCall(tcMap, tcOrder, delta1)

	// Second chunk: no index, only ID (Gemma quirk)
	delta2 := &PartialTC{
		ID:      "call_1",
		RawArgs: `"test.go"}`,
	}
	isNew, tc, tcOrder := AccumulateToolCall(tcMap, tcOrder, delta2)
	if isNew {
		t.Fatal("expected merge by ID, not new")
	}
	if tc.RawArgs != `{"file":"test.go"}` {
		t.Errorf("RawArgs = %q, want %q", tc.RawArgs, `{"file":"test.go"}`)
	}
}

func TestAccumulateToolCall_FillInLateID(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	// First chunk: index + name only (no ID yet)
	idx0 := ptrInt(0)
	delta1 := &PartialTC{
		Index: idx0,
		Name:  "bash",
	}
	_, _, tcOrder = AccumulateToolCall(tcMap, tcOrder, delta1)

	// Second chunk: fills in ID
	delta2 := &PartialTC{
		Index: ptrInt(0),
		ID:    "call_1",
	}
	_, tc, tcOrder := AccumulateToolCall(tcMap, tcOrder, delta2)
	if tc.ID != "call_1" {
		t.Errorf("ID = %q, want 'call_1'", tc.ID)
	}
	// Verify the map was updated so future lookups by ID work
	if _, ok := tcMap["call_1"]; !ok {
		t.Error("map not updated with late ID")
	}
}

func TestAccumulateToolCall_MultipleCalls(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	// First tool call
	idx0 := ptrInt(0)
	_, _, tcOrder = AccumulateToolCall(tcMap, tcOrder, &PartialTC{
		Index:   idx0,
		ID:      "call_1",
		Name:    "read_file",
		RawArgs: `{"file": "a.go"}`,
	})

	// Second tool call
	idx1 := ptrInt(1)
	_, _, tcOrder = AccumulateToolCall(tcMap, tcOrder, &PartialTC{
		Index:   idx1,
		ID:      "call_2",
		Name:    "bash",
		RawArgs: `{"command": "ls"}`,
	})

	if len(tcOrder) != 2 {
		t.Fatalf("order len = %d, want 2", len(tcOrder))
	}
	if tcOrder[0].Name != "read_file" {
		t.Errorf("first = %q, want 'read_file'", tcOrder[0].Name)
	}
	if tcOrder[1].Name != "bash" {
		t.Errorf("second = %q, want 'bash'", tcOrder[1].Name)
	}
}

func TestAccumulateToolCall_EmptyArgs(t *testing.T) {
	tcMap := make(map[string]*PartialTC)
	var tcOrder []*PartialTC

	idx := ptrInt(0)
	_, tc, tcOrder := AccumulateToolCall(tcMap, tcOrder, &PartialTC{
		Index: idx,
		ID:    "call_1",
		Name:  "no_args_tool",
		// RawArgs is empty (no parameters)
	})
	if tc.RawArgs != "" {
		t.Errorf("RawArgs = %q, want ''", tc.RawArgs)
	}
	if len(tcOrder) != 1 {
		t.Errorf("order len = %d, want 1", len(tcOrder))
	}
}

func TestToAPIToolCalls(t *testing.T) {
	idx0 := ptrInt(0)
	idx1 := ptrInt(1)
	order := []*PartialTC{
		{Index: idx0, ID: "call_1", Name: "read_file", RawArgs: `{"file":"a.go"}`},
		{Index: idx1, ID: "call_2", Name: "bash", RawArgs: `{"command":"ls"}`},
	}

	result := ToAPIToolCalls(order)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].ID != "call_1" {
		t.Errorf("first ID = %q, want 'call_1'", result[0].ID)
	}
	if result[0].Type != "function" {
		t.Errorf("first Type = %q, want 'function'", result[0].Type)
	}
}
