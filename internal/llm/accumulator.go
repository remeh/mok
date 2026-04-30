package llm

// AccumulateToolCall merges a partial tool call delta into the accumulator.
// Uses a map for fast lookup by ID and a slice to preserve insertion order.
// Returns (isNew bool, *PartialTC, []*PartialTC).
//
// Rules:
// 1. Match by index first, fall back to id (Gemma may drop index after first chunk).
// 2. Accumulate raw strings only — never parse mid-stream.
// 3. Fill in id/name late if provided in later chunks.
// 4. Default missing arguments to "".
func AccumulateToolCall(
	toolCallMap map[string]*PartialTC,
	toolCallOrder []*PartialTC,
	delta *PartialTC,
) (bool, *PartialTC, []*PartialTC) {
	if delta == nil {
		return false, nil, toolCallOrder
	}

	// Try to find existing tool call by index
	if delta.Index != nil {
		for _, tc := range toolCallOrder {
			if tc.Index != nil && *tc.Index == *delta.Index {
				// Found by index — merge
				mergePartial(toolCallMap, tc, delta)
				return false, tc, toolCallOrder
			}
		}
	}

	// Try to find existing tool call by ID
	if delta.ID != "" {
		if existing, ok := toolCallMap[delta.ID]; ok {
			mergePartial(toolCallMap, existing, delta)
			return false, existing, toolCallOrder
		}
	}

	// New tool call
	tc := &PartialTC{
		Index:   delta.Index,
		ID:      delta.ID,
		Name:    delta.Name,
		RawArgs: delta.RawArgs,
	}

	// Register in map (by ID once we have it)
	if tc.ID != "" {
		toolCallMap[tc.ID] = tc
	}

	toolCallOrder = append(toolCallOrder, tc)
	return true, tc, toolCallOrder
}

// mergePartial merges delta fields into target.
// - Accumulates RawArgs (string concatenation)
// - Fills in ID/Name if missing in target
// - Registers target in toolCallMap by ID if ID was just filled in
func mergePartial(toolCallMap map[string]*PartialTC, target *PartialTC, delta *PartialTC) {
	if target.ID == "" && delta.ID != "" {
		target.ID = delta.ID
		toolCallMap[target.ID] = target
	}
	if target.Name == "" && delta.Name != "" {
		target.Name = delta.Name
	}
	// Accumulate raw arguments (never parse mid-stream)
	target.RawArgs += delta.RawArgs
}

// ToAPIToolCalls converts accumulated PartialTCs to the wire format.
func ToAPIToolCalls(order []*PartialTC) []APIToolCall {
	result := make([]APIToolCall, 0, len(order))
	for _, tc := range order {
		result = append(result, APIToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      tc.Name,
				Arguments: tc.RawArgs,
			},
		})
	}
	return result
}
