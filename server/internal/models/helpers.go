package models

import (
	"encoding/json"
	"strings"
)

func EstimateTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += len(strings.Fields(msg.Content)) + 4
	}
	if total == 0 {
		return 8
	}
	return total
}

func ToMap(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}
