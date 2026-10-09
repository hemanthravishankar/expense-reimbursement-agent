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

func (o *OllamaClient) ExtractReceipt(
	ctx context.Context,
	pages [][]byte,
) (*models.ReceiptExtraction, error) {

	return o.extractReceiptWithModel(
		ctx,
		pages,
		VisionModel,
		false,
	)
}

func (o *OllamaClient) ExtractReceiptWithFallback(
	ctx context.Context,
	pages [][]byte,
) (*models.ReceiptExtraction, error) {

	return o.extractReceiptWithModel(
		ctx,
		pages,
		FallbackVisionModel,
		true,
	)
}

func (o *OllamaClient) extractReceiptWithModel(
	ctx context.Context,
	pages [][]byte,
	model string,
	isFallback bool,
) (*models.ReceiptExtraction, error) {

	encodedImages, err :=
		encodeImages(pages)

	if err != nil {
		return nil, err
	}

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
				Images:  encodedImages,
			},
		},

		Format: "json",
		Stream: false,
		Think:  false,

		Options: map[string]any{
			"temperature": 0,
			"num_predict": 1600,
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

func encodeImages(
	pages [][]byte,
) ([]string, error) {

	if len(pages) == 0 {
		return nil, fmt.Errorf(
			"no document pages were provided for extraction",
		)
	}

	images :=
		make(
			[]string,
			0,
			len(pages),
		)

	for index, page := range pages {

		if len(page) == 0 {
			return nil, fmt.Errorf(
				"document page %d is empty",
				index+1,
			)
		}

		images =
			append(
				images,
				base64.StdEncoding.EncodeToString(
					page,
				),
			)
	}

	return images, nil
}

func primaryExtractionPrompt() string {
	return `
Extract expense information from the supplied receipt or invoice page images.

The images may represent:
- one receipt image
- one invoice image
- multiple pages from the SAME PDF document

If multiple images are supplied, treat ALL pages as ONE expense document.

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
- use ISO currency codes.
- $ means USD.
- ₹ means INR.
- € means EUR.
- £ means GBP.
- missing string = "".
- missing number = 0.
- missing array = [].
- confidence must be between 0 and 1.

For multi-page documents:

- combine information across all pages.
- return ONE expense object for the entire document.
- do not count repeated totals more than once.
- use the final payable/grand total for the entire document.
- line items may appear across multiple pages.
- do not treat page headers, footers or carried-forward totals as line items.

Line item rules:

- extract only genuine purchased items.
- do not treat modifiers or preparation instructions as separate items.
- do not assign a nearby item's price to an unpriced line.
- if a line has no standalone visible price, normally treat it as
  a modifier unless clearly shown as a purchased item.

Examples of modifiers:

- no bun
- no ketchup
- medium
- medium rare
- sauce on side
- substitutions
- replacement bread
- cooking instructions

Before returning:

- verify subtotal.
- verify tax.
- verify total.
- check whether line item amounts are consistent with subtotal.

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
Perform a completely fresh extraction of the supplied receipt or invoice.

The supplied images may be multiple pages from ONE document.

A previous automated extraction failed deterministic validation.

IMPORTANT:

- independently inspect every supplied page.
- ignore any previous extraction.
- extract only visible information.
- do not invent values to make calculations match.
- carefully distinguish purchased items from modifiers.
- do not assign neighboring prices to unpriced lines.
- combine multiple pages into ONE expense.
- do not double-count repeated subtotal or total values.

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

Rules:

- expense_date must use YYYY-MM-DD.
- monetary values must be JSON numbers.
- $ = USD.
- ₹ = INR.
- € = EUR.
- £ = GBP.
- missing string = "".
- missing number = 0.
- missing array = [].
- confidence must be between 0 and 1.

For multi-page documents:

- use all pages as one document.
- use the final grand/payable total.
- do not add totals repeated on different pages.
- extract line items across pages where possible.

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
