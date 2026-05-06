package llm

// Refiner is the common interface satisfied by every LLM provider client.
// Adding a new provider only requires implementing this interface and
// registering it in the App — no changes to the pipeline are needed.
type Refiner interface {
	// RefineText sends raw transcription text to the LLM for cleanup.
	// Returns (polishedText, completionTokenCount, okToGo, error).
	// okToGo == true means the LLM signalled the raw text is already clean
	// and should be used as-is.
	RefineText(rawText, model string) (string, int, bool, error)

	// CheckModel runs a latency probe against the provider and returns
	// (latencyMs, tokensPerSecond, error).
	CheckModel(model string) (int64, float64, error)
}
