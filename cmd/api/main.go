package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/ai"
	expenseapi "github.com/hemanth-shankar/expense-agent/internal/api"
	"github.com/hemanth-shankar/expense-agent/internal/persistence"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

func main() {

	ctx :=
		context.Background()

	//
	// PostgreSQL is initialized before the workflow because
	// it is also the source of the currently published
	// reimbursement policy.
	//
	databaseURL :=
		os.Getenv(
			"DATABASE_URL",
		)

	if databaseURL == "" {

		log.Fatal(
			"DATABASE_URL environment variable is required",
		)
	}

	repository, err :=
		persistence.NewPostgresRepository(
			ctx,
			databaseURL,
		)

	if err != nil {

		log.Fatalf(
			"failed to initialize PostgreSQL: %v",
			err,
		)
	}

	defer repository.Close()

	log.Println(
		"PostgreSQL connection established",
	)

	//
	// AI client.
	//
	client :=
		ai.NewOllamaClient()

	//
	// Expense processing workflow.
	//
	// Every new claim resolves the active policy from
	// PostgreSQL at decision time, so publishing a new
	// policy version does not require restarting/rebuilding
	// the API server.
	//
	workflowConfig :=
		workflow.DefaultConfig()

	workflowConfig.PolicyProvider =
		repository

	processor :=
		workflow.NewProcessor(
			client,
			workflowConfig,
		)

	//
	// HTTP API.
	//
	handler :=
		expenseapi.NewHandlerWithRepository(
			processor,
			repository,
		)

	mux :=
		http.NewServeMux()

	handler.RegisterRoutes(
		mux,
	)

	server :=
		&http.Server{
			Addr: ":8080",

			Handler: expenseapi.WithCORS(
				mux,
			),

			ReadHeaderTimeout: 5 * time.Second,
		}

	log.Println(
		"Expense Agent API listening on http://localhost:8080",
	)

	if err :=
		server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {

		log.Fatal(err)
	}
}
