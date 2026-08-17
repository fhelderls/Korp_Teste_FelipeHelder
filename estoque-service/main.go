package main

import (
	"log"
	"net/http"
	"os"

	"korp-teste/estoque-service/db"
	"korp-teste/estoque-service/handlers"
	"korp-teste/estoque-service/store"
)

func main() {

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("variavel de ambiente DATABASE_URL nao foi definida")

	}
	database, err := db.Connect(connString)
	if err != nil {
		log.Fatalf("falha ao conectar ao banco de dados: %v", err)
	}
	defer database.Close()

	if err := db.InitSchema(database); err != nil {
		log.Fatalf("falha ao iniciar o schema: %v", err)
	}

	produtosStore := store.NewProdutosStore(database)
	reservasStore := store.NewReservasStore(database)

	produtosHandlers := handlers.NewProdutosHandlers(produtosStore)
	reservasHandlers := handlers.NewReservasHandlers(reservasStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /produtos", produtosHandlers.List)
	mux.HandleFunc("POST /produtos", produtosHandlers.Create)
	mux.HandleFunc("POST /reservas", reservasHandlers.Reservar)
	mux.HandleFunc("POST /reservas/{chave}/confirmar", reservasHandlers.Confirmar)
	mux.HandleFunc("POST /reservas/{chave}/cancelar", reservasHandlers.Cancelar)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("estoque-service rodando na porta %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("falha ao iniciar servidor: %v", err)

	}
}
