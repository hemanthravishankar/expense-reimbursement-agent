package main

import (
	"log"
	"net/http"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/ai"
	expenseapi "github.com/hemanth-shankar/expense-agent/internal/api"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

func main() {

	client :=
		ai.NewOllamaClient()

	processor :=
		workflow.NewProcessor(
			client,
			workflow.DefaultConfig(),
		)

	handler :=
		expenseapi.NewHandler(
			processor,
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
