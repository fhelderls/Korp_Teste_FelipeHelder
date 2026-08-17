package main

import (
	"log"
	"net/http"
	"os"

	"korp-teste/faturamento-service/client"
	"korp-teste/faturamento-service/db"
	"korp-teste/faturamento-service/handlers"
	"korp-teste/faturamento-service/store"
)

func main() {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("variavel de ambiente DATABASE_URL nao definida")
	}

	database, err := db.Connect(connString)
	if err != nil {
		log.Fatalf("falha ao conectar ao banco de dados: %v", err)
	}
	defer database.Close()

	if err := db.InitSchema(database); err != nil {
		log.Fatalf("falha ao inicializar schema: %v", err)
	}

	estoqueURL := os.Getenv("ESTOQUE_SERVICE_URL")
	if estoqueURL == "" {
		log.Fatal("variavel de ambiente ESTOQUE_SERVICE_URL nao definida")
	}

	notasStore := store.NewNotasStore(database)
	estoqueClient := client.NewEstoqueClient(estoqueURL)
	notasHandlers := handlers.NewNotasHandlers(notasStore, estoqueClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /notas", notasHandlers.List)
	mux.HandleFunc("POST /notas", notasHandlers.Emitir)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("faturamento-service rodando na porta %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("falha ao iniciar servidor: %v", err)
	}
}
