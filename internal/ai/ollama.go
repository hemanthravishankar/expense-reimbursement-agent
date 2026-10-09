package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

const (
	OllamaURL           = "http://localhost:11434/api/chat"
	VisionModel         = "qwen3-vl:4b-instruct"
	FallbackVisionModel = "qwen3-vl:8b-instruct"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

type chatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Format   string         `json:"format,omitempty"`
	Stream   bool           `json:"stream"`
	Think    bool           `json:"think"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatResponse struct {
	Model string `json:"model"`

	Message struct {
		Role     string `json:"role"`
		Content  string `json:"content"`
		Thinking string `json:"thinking,omitempty"`
	} `json:"message"`

	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason"`
}

func NewOllamaClient() *OllamaClient {
	return &OllamaClient{
		BaseURL: OllamaURL,
		Model:   VisionModel,

		Client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Generate is a simple text-only call.
// We can use this later for policy explanations,
// clarification questions, summaries, etc.
func (o *OllamaClient) Generate(
	ctx context.Context,
	prompt string,
) (string, error) {

	requestBody := chatRequest{
		Model: o.Model,

		Messages: []chatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},

		Stream: false,
		Think:  false,

		Options: map[string]any{
			"temperature": 0,
			"num_predict": 500,
		},
	}

	return o.call(ctx, requestBody)
}

// ExtractReceipt performs the normal receipt extraction
// using the smaller/faster 4B model.
func (o *OllamaClient) ExtractReceipt(
	ctx context.Context,
	imageData []byte,
) (*models.ReceiptExtraction, error) {

	return o.extractReceiptWithModel(
		ctx,
		imageData,
		VisionModel,
		false,
	)
}

// ExtractReceiptWithFallback performs a completely fresh
// extraction using the stronger 8B model.
//
// We deliberately DO NOT send the first model's output
// to the fallback model. This reduces anchoring on the
// original model's mistake.
func (o *OllamaClient) ExtractReceiptWithFallback(
	ctx context.Context,
	imageData []byte,
) (*models.ReceiptExtraction, error) {

	return o.extractReceiptWithModel(
		ctx,
		imageData,
		FallbackVisionModel,
		true,
	)
}

func (o *OllamaClient) extractReceiptWithModel(
	ctx context.Context,
	imageData []byte,
	model string,
	isFallback bool,
) (*models.ReceiptExtraction, error) {

	encodedImage :=
		base64.StdEncoding.EncodeToString(imageData)

	prompt := primaryExtractionPrompt()

	if isFallback {
		prompt = fallbackExtractionPrompt()
	}

	requestBody := chatRequest{
		Model: model,

		Messages: []chatMessage{
			{
				Role:    "user",
				Content: prompt,

				Images: []string{
					encodedImage,
				},
			},
		},

		Format: "json",
		Stream: false,
		Think:  false,

		Options: map[string]any{
			"temperature": 0,
			"num_predict": 1200,
		},
	}

	rawResponse, err :=
		o.call(ctx, requestBody)

	if err != nil {
		return nil, fmt.Errorf(
			"receipt extraction using %s failed: %w",
			model,
			err,
		)
	}

	if rawResponse == "" {
		return nil, fmt.Errorf(
			"model %s returned an empty receipt extraction response",
			model,
		)
	}

	var extraction models.ReceiptExtraction

	if err := json.Unmarshal(
		[]byte(rawResponse),
		&extraction,
	); err != nil {

		return nil, fmt.Errorf(
			"decode receipt extraction JSON from %s: %w\nraw response: %s",
			model,
			err,
			rawResponse,
		)
	}

	return &extraction, nil
}

func primaryExtractionPrompt() string {
	return `
Extract expense information from this receipt image.

Return ONLY valid JSON.
Do not explain your reasoning.
Do not return markdown.
Do not infer information that is not visible.

Use exactly this structure:

{
  "document_type": "receipt",
  "merchant": "",
  "expense_date": "",
  "subtotal": 0,
  "tax_amount": 0,
  "total_amount": 0,
  "currency": "",
  "receipt_number": "",
  "payment_method": "",
  "category_suggestion": "",
  "line_items": [
    {
      "name": "",
      "quantity": 0,
      "amount": 0
    }
  ],
  "confidence": 0,
  "warnings": []
}

General rules:

- expense_date must use YYYY-MM-DD.
- monetary fields must be JSON numbers.
- use ISO currency codes such as USD, INR, EUR and GBP.
- $ means USD.
- ₹ means INR.
- € means EUR.
- £ means GBP.
- missing string values must be "".
- missing numeric values must be 0.
- missing arrays must be [].
- confidence must be between 0 and 1.

Line item rules:

- extract only purchased items.
- do not treat modifiers as separate purchased items.
- do not treat preparation instructions as purchased items.
- do not assign the price of a nearby item to an unpriced line.
- if a line has no standalone visible price, normally treat it as
  a modifier unless it is clearly a separate purchased item.

Examples of modifiers:

- no bun
- no ketchup
- no bread
- medium
- medium rare
- sauce on side
- substitutions
- replacement bread
- cooking instructions

Before returning JSON:

- carefully verify the visible subtotal.
- carefully verify the visible tax.
- carefully verify the visible total.
- check whether the actual purchased line items are consistent
  with the visible subtotal.

Allowed categories:

Accommodation
Meals
Transportation
Travel
Office Expense
Internet
Mobile
Entertainment
Other
`
}

func fallbackExtractionPrompt() string {
	return `
Perform a completely fresh extraction of this receipt.

A previous automated extraction failed deterministic validation.

IMPORTANT:

- Ignore any previous extraction.
- Independently inspect the ORIGINAL receipt image.
- Extract only information that is actually visible.
- Do not invent values simply to make calculations match.
- Pay special attention to line items and their prices.
- Carefully distinguish purchased items from modifiers,
  substitutions and preparation instructions.
- Do not assign a neighboring item's price to an unpriced line.

Return ONLY valid JSON.
Do not explain your reasoning.
Do not return markdown.

Use exactly this structure:

{
  "document_type": "receipt",
  "merchant": "",
  "expense_date": "",
  "subtotal": 0,
  "tax_amount": 0,
  "total_amount": 0,
  "currency": "",
  "receipt_number": "",
  "payment_method": "",
  "category_suggestion": "",
  "line_items": [
    {
      "name": "",
      "quantity": 0,
      "amount": 0
    }
  ],
  "confidence": 0,
  "warnings": []
}

General rules:

- expense_date must use YYYY-MM-DD.
- monetary fields must be JSON numbers.
- $ means USD.
- ₹ means INR.
- € means EUR.
- £ means GBP.
- missing string values must be "".
- missing numeric values must be 0.
- missing arrays must be [].
- confidence must be between 0 and 1.

Line item rules:

- include only genuine purchased items.
- an unpriced modifier must NOT inherit the price of the next item.
- an unpriced modifier must NOT inherit the price of the previous item.
- a substitution or preparation instruction is not a separate
  purchase unless it clearly has its own price.

Examples of modifiers:

- no bun
- no ketchup
- no bread
- medium
- medium rare
- sauce on side
- sub sourdough
- substitutions
- preparation instructions

Before returning:

1. identify the visible subtotal.
2. identify the purchased line items.
3. independently verify whether the line item amounts are
   consistent with that subtotal.
4. do not manufacture values to make them match.

Allowed categories:

Accommodation
Meals
Transportation
Travel
Office Expense
Internet
Mobile
Entertainment
Other
`
}

func (o *OllamaClient) call(
	ctx context.Context,
	requestBody chatRequest,
) (string, error) {

	data, err :=
		json.Marshal(requestBody)

	if err != nil {
		return "", fmt.Errorf(
			"marshal Ollama request: %w",
			err,
		)
	}

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			o.BaseURL,
			bytes.NewReader(data),
		)

	if err != nil {
		return "", fmt.Errorf(
			"create Ollama request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err :=
		o.Client.Do(req)

	if err != nil {
		return "", fmt.Errorf(
			"call Ollama: %w",
			err,
		)
	}

	defer resp.Body.Close()

	body, err :=
		io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf(
			"read Ollama response: %w",
			err,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"Ollama returned HTTP status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var result chatResponse

	if err := json.Unmarshal(
		body,
		&result,
	); err != nil {

		return "", fmt.Errorf(
			"decode Ollama API response: %w\nraw response: %s",
			err,
			string(body),
		)
	}

	if result.Message.Content == "" {
		return "", fmt.Errorf(
			"Ollama model returned empty content; model=%s done=%v done_reason=%q thinking=%q raw_response=%s",
			requestBody.Model,
			result.Done,
			result.DoneReason,
			result.Message.Thinking,
			string(body),
		)
	}

	return result.Message.Content, nil
}
