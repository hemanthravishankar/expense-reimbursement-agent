package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hemanth-shankar/expense-agent/internal/ai"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

func main() {

	if len(os.Args) != 2 {

		log.Fatalf(
			"usage: go run ./cmd/server <receipt-file>",
		)
	}

	documentPath :=
		os.Args[1]

	documentData, err :=
		os.ReadFile(
			documentPath,
		)

	if err != nil {

		log.Fatalf(
			"failed to read receipt document: %v",
			err,
		)
	}

	employee :=
		models.Employee{
			ID:         "EMP10042",
			Name:       "Ananya Rao",
			Grade:      "B2",
			Department: "Engineering",
			Location:   "Bengaluru",
		}

	client :=
		ai.NewOllamaClient()

	processor :=
		workflow.NewProcessor(
			client,
			workflow.DefaultConfig(),
		)

	fmt.Println(
		"Processing expense claim...",
	)

	fmt.Println(
		"Document:",
		documentPath,
	)

	fmt.Println(
		"Employee:",
		employee.Name,
	)

	result, err :=
		processor.ProcessReceipt(
			context.Background(),
			documentPath,
			documentData,
			employee,
		)

	if err != nil {
		log.Fatal(err)
	}

	printResult(
		result,
	)
}

func printResult(
	result *workflow.Result,
) {

	fmt.Println()
	fmt.Println(
		"=== EXPENSE AGENT RESULT ===",
	)

	fmt.Println(
		"Run ID:",
		result.RunID,
	)

	fmt.Println(
		"Document Type:",
		result.DocumentMIMEType,
	)

	fmt.Println(
		"Pages:",
		result.PageCount,
	)

	fmt.Println(
		"SHA-256:",
		result.ReceiptSHA256,
	)

	fmt.Println(
		"AI Processing Skipped:",
		result.AIProcessingSkipped,
	)

	if result.AIProcessingSkipped {

		fmt.Println()

		fmt.Println(
			"Exact duplicate detected before AI extraction.",
		)

		fmt.Println(
			"Vision model was not executed.",
		)

	} else {

		fmt.Println(
			"Final Model:",
			result.FinalModel,
		)

		fmt.Println(
			"Fallback Used:",
			result.FallbackUsed,
		)

		if result.FinalExtraction != nil {

			printJSON(
				"Receipt Extraction",
				result.FinalExtraction,
			)
		}

		printJSON(
			"Validation",
			result.FinalValidation,
		)
	}

	if result.DuplicateResult != nil {

		printJSON(
			"Duplicate Detection",
			result.DuplicateResult,
		)
	}

	if result.ClarificationResult != nil &&
		result.ClarificationResult.Required {

		printJSON(
			"Clarification",
			result.ClarificationResult,
		)

		fmt.Println()
		fmt.Println(
			"Questions for Employee:",
		)

		for i, question := range result.ClarificationResult.Questions {

			fmt.Printf(
				"%d. %s\n",
				i+1,
				question.Question,
			)
		}
	}

	if result.PolicyDecision != nil {

		printJSON(
			"Policy Decision",
			result.PolicyDecision,
		)
	}

	if len(result.Warnings) > 0 {

		fmt.Println()
		fmt.Println(
			"Workflow Warnings:",
		)

		for _, warning := range result.Warnings {

			fmt.Println(
				"-",
				warning,
			)
		}
	}

	fmt.Println()

	fmt.Println(
		"FINAL CLAIM STATUS:",
		result.FinalStatus,
	)

	fmt.Println(
		"Audit Record:",
		result.AuditPath,
	)
}

func printJSON(
	title string,
	value any,
) {

	output, err :=
		json.MarshalIndent(
			value,
			"",
			"  ",
		)

	if err != nil {

		log.Printf(
			"failed to format %s: %v",
			title,
			err,
		)

		return
	}

	fmt.Println()
	fmt.Println(title + ":")
	fmt.Println(string(output))
}
