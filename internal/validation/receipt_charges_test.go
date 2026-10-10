package validation

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestReceiptWithServiceChargeIsValid(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			DocumentType: "receipt",

			Merchant: "THE DAILY GRILL",

			ExpenseDate: "2024-02-12",

			Subtotal: 48,

			TaxAmount: 2.40,

			ServiceCharge: 4.80,

			TotalAmount: 55.20,

			Currency: "USD",

			ReceiptNumber: "78452",

			CategorySuggestion: "Meals",

			Confidence: 0.98,

			LineItems: []models.ReceiptLineItem{
				{
					Name: "Grilled Chicken Salad",

					Quantity: 1,

					Amount: 18,
				},
				{
					Name: "Pasta Alfredo",

					Quantity: 1,

					Amount: 16,
				},
				{
					Name: "Iced Tea",

					Quantity: 2,

					Amount: 6,
				},
				{
					Name: "Tiramisu",

					Quantity: 1,

					Amount: 8,
				},
			},
		}

	result :=
		ValidateReceipt(
			receipt,
		)

	if !result.Valid {
		t.Fatalf(
			"expected receipt to be valid, got issues: %+v",
			result.Issues,
		)
	}

	if result.LineItemSum != 48 {
		t.Fatalf(
			"expected line-item sum 48.00, got %.2f",
			result.LineItemSum,
		)
	}
}

func TestReceiptWithTipAndDiscountIsValid(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			Merchant: "Demo Restaurant",

			ExpenseDate: "2026-10-10",

			Subtotal: 100,

			TaxAmount: 10,

			TipAmount: 15,

			DiscountAmount: 5,

			TotalAmount: 120,

			Currency: "USD",

			Confidence: 0.99,
		}

	result :=
		ValidateReceipt(
			receipt,
		)

	if !result.Valid {
		t.Fatalf(
			"expected receipt to be valid, got issues: %+v",
			result.Issues,
		)
	}
}

func TestReceiptWithUnexplainedDifferenceFails(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			Merchant: "Demo Restaurant",

			ExpenseDate: "2026-10-10",

			Subtotal: 48,

			TaxAmount: 2.40,

			TotalAmount: 55.20,

			Currency: "USD",

			Confidence: 0.99,
		}

	result :=
		ValidateReceipt(
			receipt,
		)

	if result.Valid {
		t.Fatal(
			"expected unexplained total mismatch to fail validation",
		)
	}
}
