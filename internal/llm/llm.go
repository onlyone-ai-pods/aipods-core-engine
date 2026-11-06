// Package llm connects the core to a real language model (SPEC-CORE-66).
package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoText is returned when the model answers without a text block.
var ErrNoText = errors.New("llm: response has no text")

// ErrRefused is returned when the model declines the request.
var ErrRefused = errors.New("llm: request refused by the model")

// Option is one choice the classifier can pick (e.g. a pod).
type Option struct {
	ID          string
	Description string
}

// Effort controls how much the model reasons before answering.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
)

// Request is a single-turn completion request.
type Request struct {
	System    string
	User      string
	MaxTokens int64
	Effort    Effort
}

// Client is the minimal surface the core needs from an LLM provider.
type Client interface {
	Complete(ctx context.Context, req Request) (string, error)
	Model() string
}

const classifySystem = `Sos el clasificador de intenciones de AI Pods. Recibís una consulta de un cliente y una lista cerrada de AI Pods.
Respondé únicamente con el ID exacto del pod que mejor atiende la consulta, sin explicación ni puntuación.
Si ninguno encaja, respondé NONE.`

// Classify asks the model to pick one option ID. It returns an error if the
// answer is not one of the given IDs, so callers can fall back safely.
func Classify(ctx context.Context, c Client, query string, options []Option) (string, error) {
	if len(options) == 0 {
		return "", errors.New("llm: no options to classify")
	}
	var b strings.Builder
	b.WriteString("AI Pods disponibles:\n")
	for _, o := range options {
		fmt.Fprintf(&b, "- %s: %s\n", o.ID, o.Description)
	}
	b.WriteString("\nConsulta del cliente:\n<consulta>\n")
	b.WriteString(query)
	b.WriteString("\n</consulta>")

	out, err := c.Complete(ctx, Request{
		System:    classifySystem,
		User:      b.String(),
		MaxTokens: 2000,
		Effort:    EffortLow,
	})
	if err != nil {
		return "", err
	}
	answer := strings.Trim(strings.TrimSpace(out), "`'\". \n")
	for _, o := range options {
		if strings.EqualFold(answer, o.ID) {
			return o.ID, nil
		}
	}
	return "", fmt.Errorf("llm: classifier answered %q, not a known option", answer)
}
