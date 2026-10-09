package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hemanth-shankar/expense-agent/internal/ai"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf(
			"usage: go run ./cmd/server <receipt-image>",
		)
	}

	imagePath := os.Args[1]

	imageData, err :=
		os.ReadFile(imagePath)

	if err != nil {
		log.Fatalf(
			"failed to read receipt image: %v",
			err,
		)
	}

	fmt.Println("Analyzing receipt...")
	fmt.Println("File:", imagePath)

	client := ai.NewOllamaClient()

	//
	// STEP 1:
	// Primary extraction using Qwen3-VL 4B.
	//
	fmt.Println()
	fmt.Println(
		"Running primary extraction with Qwen3-VL 4B...",
	)

	extraction, err :=
		client.ExtractReceipt(
			context.Background(),
			imageData,
		)

	if err != nil {
		log.Fatal(err)
	}

	//
	// STEP 2:
	// Normalize values produced by AI.
	//
	validation.NormalizeReceipt(
		extraction,
	)

	//
	// STEP 3:
	// Deterministically validate the extraction.
	//
	validationResult :=
		validation.ValidateReceipt(
			extraction,
		)

	//
	// Keep track of which model produced
	// the final accepted extraction.
	//
	finalModel := "qwen3-vl:4b-instruct"

	//
	// STEP 4:
	// If primary extraction fails validation,
	// escalate to Qwen3-VL 8B.
	//
	if !validationResult.Valid {

		fmt.Println()
		fmt.Println(
			"Primary extraction failed validation.",
		)

		printValidationIssues(
			validationResult,
		)

		fmt.Println()
		fmt.Println(
			"Escalating to Qwen3-VL 8B for fresh extraction...",
		)

		fallbackExtraction, fallbackErr :=
			client.ExtractReceiptWithFallback(
				context.Background(),
				imageData,
			)

		if fallbackErr != nil {

			log.Printf(
				"Fallback extraction failed: %v",
				fallbackErr,
			)

		} else {

			//
			// Normalize the 8B result.
			//
			validation.NormalizeReceipt(
				fallbackExtraction,
			)

			//
			// Validate the 8B result.
			//
			fallbackValidation :=
				validation.ValidateReceipt(
					fallbackExtraction,
				)

			//
			// Replace the current result with
			// the fallback result.
			//
			extraction =
				fallbackExtraction

			validationResult =
				fallbackValidation

			finalModel =
				"qwen3-vl:8b-instruct"
		}
	}

	//
	// STEP 5:
	// Print final extraction.
	//
	printFinalExtraction(
		extraction,
		finalModel,
	)

	//
	// STEP 6:
	// Print final validation result.
	//
	printFinalValidation(
		validationResult,
	)

	//
	// STEP 7:
	// Give a human-readable final status.
	//
	fmt.Println()

	if validationResult.Valid {

		fmt.Println(
			"STATUS: RECEIPT VALIDATED",
		)

		fmt.Println(
			"The extracted receipt passed deterministic validation.",
		)

	} else {

		fmt.Println(
			"STATUS: HUMAN REVIEW REQUIRED",
		)

		fmt.Println(
			"The receipt still contains validation errors after AI escalation.",
		)
	}
}

func printFinalExtraction(
	extraction *models.ReceiptExtraction,
	model string,
) {

	output, err :=
		json.MarshalIndent(
			extraction,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println(
		"Final Receipt Extraction:",
	)

	fmt.Println(
		"Model:",
		model,
	)

	fmt.Println(
		string(output),
	)
}

func printFinalValidation(
	result validation.ReceiptValidationResult,
) {

	validationJSON, err :=
		json.MarshalIndent(
			result,
			"",
			"  ",
		)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println()
	fmt.Println(
		"Final Validation Result:",
	)

	fmt.Println(
		string(validationJSON),
	)
}

func printValidationIssues(
	result validation.ReceiptValidationResult,
) {

	for _, issue := range result.Issues {

		fmt.Printf(
			"- [%s] %s: %s\n",
			issue.Severity,
			issue.Code,
			issue.Message,
		)
	}
}
