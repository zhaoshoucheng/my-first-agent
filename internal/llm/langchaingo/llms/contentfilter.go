package llms

// FilterResult represents the content filter results
// err:
// API returned unexpected status code: 400: The response was filtered due to the prompt triggering Azure
// OpenAI's content management policy. Please modify your prompt and retry. To learn more about our
// content filtering policies please read our documentation: https://go.microsoft.com/fwlink/?linkid=2198766
type FilterResult struct {
	Jailbreak bool `json:"jailbreak"`
}

// ExtractFilterResult extracts filter results from generation info
func ExtractFilterResult(generationInfo map[string]any) FilterResult {
	filter := FilterResult{}
	if len(generationInfo) == 0 {
		return filter
	}

	// Extract FilterResult if present
	if filterResult, ok := generationInfo["FilterResult"]; ok {
		if filterMap, ok := filterResult.(map[string]any); ok {
			if jailbreak, exists := filterMap["jailbreak"]; exists {
				filter.Jailbreak = jailbreak.(bool)
			}
		}
	}
	return filter
}
