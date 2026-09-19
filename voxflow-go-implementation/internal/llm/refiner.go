package llm

// New providers implement this; pipeline stays unchanged.
type Refiner interface {
	RefineText(rawText, model string) (string, int, bool, error)

	CheckModel(model string) (int64, float64, error)

	RetryWithInstruction(text, instruction, model string) (string, error)

	Prewarm(model string)
}
