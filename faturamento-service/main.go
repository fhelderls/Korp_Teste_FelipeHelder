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

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

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

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicAPIKey == "" {
		log.Println("aviso: ANTHROPIC_API_KEY nao definida, o resumo com IA nao vai funcionar")
	}

	notasStore := store.NewNotasStore(database)
	estoqueClient := client.NewEstoqueClient(estoqueURL)
	aiClient := client.NewAnthropicClient(anthropicAPIKey)
	notasHandlers := handlers.NewNotasHandlers(notasStore, estoqueClient, aiClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /notas", notasHandlers.List)
	mux.HandleFunc("POST /notas", notasHandlers.Criar)
	mux.HandleFunc("POST /notas/{chave}/imprimir", notasHandlers.Imprimir)
	mux.HandleFunc("DELETE /notas/{chave}", notasHandlers.Cancelar)
	mux.HandleFunc("GET /notas/resumo", notasHandlers.Resumo)
	mux.HandleFunc("GET /notas/relatorio", notasHandlers.Relatorio)
	mux.HandleFunc("GET /notas/{chave}/pdf", notasHandlers.PDF)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("faturamento-service rodando na porta %s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {

		log.Fatalf("falha ao iniciar servidor: %v", err)
	}
}
