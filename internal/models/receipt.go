package models

type ReceiptLineItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Amount   float64 `json:"amount"`
}

type ReceiptExtraction struct {
	DocumentType       string            `json:"document_type"`
	Merchant           string            `json:"merchant"`
	ExpenseDate        string            `json:"expense_date"`
	Subtotal           float64           `json:"subtotal"`
	TaxAmount          float64           `json:"tax_amount"`
	ServiceCharge      float64           `json:"service_charge"`
	TipAmount          float64           `json:"tip_amount"`
	DiscountAmount     float64           `json:"discount_amount"`
	OtherFees          float64           `json:"other_fees"`
	TotalAmount        float64           `json:"total_amount"`
	Currency           string            `json:"currency"`
	ReceiptNumber      string            `json:"receipt_number"`
	PaymentMethod      string            `json:"payment_method"`
	CategorySuggestion string            `json:"category_suggestion"`
	LineItems          []ReceiptLineItem `json:"line_items"`
	Confidence         float64           `json:"confidence"`
	Warnings           []string          `json:"warnings"`
}
