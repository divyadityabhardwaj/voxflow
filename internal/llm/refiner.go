package llm

import "context"

// New providers implement this; pipeline stays unchanged.
type Refiner interface {
	RefineTextContext(ctx context.Context, rawText, model string) (string, int, bool, error)
	RetryWithInstructionContext(ctx context.Context, text, instruction, model string) (string, error)
	RefineText(rawText, model string) (string, int, bool, error)

	CheckModel(model string) (int64, float64, error)

	RetryWithInstruction(text, instruction, model string) (string, error)

	Prewarm(model string)
}
