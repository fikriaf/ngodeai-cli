package cost

import (
	"fmt"
	"strings"
	"time"
)

// Pricing data for common models (per 1M tokens)
var ModelPricing = map[string]Pricing{
	// OpenAI Models
	"gpt-4o": {
		InputPer1M:  2.50,
		OutputPer1M: 10.00,
	},
	"gpt-4o-mini": {
		InputPer1M:  0.15,
		OutputPer1M: 0.60,
	},
	"gpt-4-turbo": {
		InputPer1M:  10.00,
		OutputPer1M: 30.00,
	},
	"gpt-4": {
		InputPer1M:  30.00,
		OutputPer1M: 60.00,
	},
	"gpt-3.5-turbo": {
		InputPer1M:  0.50,
		OutputPer1M: 1.50,
	},
	
	// Anthropic Models
	"claude-3-5-sonnet-20241022": {
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
	},
	"claude-3-5-sonnet": {
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
	},
	"claude-3-opus-20240229": {
		InputPer1M:  15.00,
		OutputPer1M: 75.00,
	},
	"claude-3-opus": {
		InputPer1M:  15.00,
		OutputPer1M: 75.00,
	},
	"claude-3-haiku-20240307": {
		InputPer1M:  0.25,
		OutputPer1M: 1.25,
	},
	"claude-3-haiku": {
		InputPer1M:  0.25,
		OutputPer1M: 1.25,
	},
	
	// Google Models
	"gemini-1.5-pro": {
		InputPer1M:  1.25,
		OutputPer1M: 5.00,
	},
	"gemini-1.5-flash": {
		InputPer1M:  0.075,
		OutputPer1M: 0.30,
	},
	"gemini-pro": {
		InputPer1M:  0.50,
		OutputPer1M: 1.50,
	},
}

// Pricing represents cost per 1M tokens
type Pricing struct {
	InputPer1M  float64 `json:"input_per_1m"`
	OutputPer1M float64 `json:"output_per_1m"`
}

// Usage represents token usage for a single request
type Usage struct {
	Model          string    `json:"model"`
	PromptTokens   int64     `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens    int64     `json:"total_tokens"`
	InputCost      float64   `json:"input_cost"`
	OutputCost     float64   `json:"output_cost"`
	TotalCost      float64   `json:"total_cost"`
	Timestamp      time.Time `json:"timestamp"`
}

// Tracker tracks costs across multiple requests
type Tracker struct {
	usages []Usage
}

// NewTracker creates a new cost tracker
func NewTracker() *Tracker {
	return &Tracker{
		usages: make([]Usage, 0),
	}
}

// GetPricing returns pricing for a model, or default pricing if unknown
func GetPricing(model string) Pricing {
	if p, ok := ModelPricing[model]; ok {
		return p
	}
	
	// Try to match partial model names
	for key, pricing := range ModelPricing {
		if contains(model, key) || contains(key, model) {
			return pricing
		}
	}
	
	// Default pricing for unknown models
	return Pricing{
		InputPer1M:  5.00,
		OutputPer1M: 15.00,
	}
}

// CalculateCost calculates cost for a single request
func CalculateCost(model string, promptTokens, completionTokens int64) Usage {
	pricing := GetPricing(model)
	
	inputCost := float64(promptTokens) * pricing.InputPer1M / 1_000_000
	outputCost := float64(completionTokens) * pricing.OutputPer1M / 1_000_000
	totalCost := inputCost + outputCost
	
	return Usage{
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		InputCost:        inputCost,
		OutputCost:       outputCost,
		TotalCost:        totalCost,
		Timestamp:        time.Now(),
	}
}

// Add adds a usage record
func (t *Tracker) Add(usage Usage) {
	t.usages = append(t.usages, usage)
}

// AddRequest calculates and adds a usage record
func (t *Tracker) AddRequest(model string, promptTokens, completionTokens int64) Usage {
	usage := CalculateCost(model, promptTokens, completionTokens)
	t.Add(usage)
	return usage
}

// GetTotalCost returns total cost across all requests
func (t *Tracker) GetTotalCost() float64 {
	total := 0.0
	for _, usage := range t.usages {
		total += usage.TotalCost
	}
	return total
}

// GetTotalTokens returns total tokens across all requests
func (t *Tracker) GetTotalTokens() (input, output, total int64) {
	for _, usage := range t.usages {
		input += usage.PromptTokens
		output += usage.CompletionTokens
		total += usage.TotalTokens
	}
	return
}

// GetUsage returns all usage records
func (t *Tracker) GetUsage() []Usage {
	return t.usages
}

// GetLastUsage returns the most recent usage
func (t *Tracker) GetLastUsage() *Usage {
	if len(t.usages) == 0 {
		return nil
	}
	return &t.usages[len(t.usages)-1]
}

// Reset clears all usage records
func (t *Tracker) Reset() {
	t.usages = make([]Usage, 0)
}

// FormatCost formats a cost value for display
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return "$0.00"
	}
	if cost < 1.00 {
		return formatCents(cost)
	}
	return formatDollars(cost)
}

func formatCents(cost float64) string {
	cents := cost * 100
	if cents < 1 {
		return "< $0.01"
	}
	return fmt.Sprintf("%.2f¢", cents)
}

func formatDollars(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
