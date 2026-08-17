package store

import (
	"database/sql"
	"os"
	"testing"

	"korp-teste/estoque-service/db"
	"korp-teste/estoque-service/models"
)

var testDB *sql.DB

// TestMain roda uma vez antes de todos os testes do pacote: conecta no
// banco de teste (o mesmo Postgres do docker-compose) e garante que as
// tabelas existem antes de qualquer teste rodar.
func TestMain(m *testing.M) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgres://korp:korp@localhost:5435/estoque?sslmode=disable"
	}

	database, err := db.Connect(connString)
	if err != nil {
		panic("falha ao conectar ao banco de teste: " + err.Error())
	}
	if err := db.InitSchema(database); err != nil {
		panic("falha ao inicializar schema de teste: " + err.Error())
	}

	testDB = database
	code := m.Run()
	database.Close()
	os.Exit(code)
}

func TestReservar_DescontaSaldo(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_RESERVAR_1", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_1")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 4}}
	if err := reservasStore.Reservar("TEST_CHAVE_1", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 6 {
		t.Errorf("saldo esperado 6, obtido %d", atualizado.Saldo)
	}
}

func TestReservar_SaldoInsuficiente(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_RESERVAR_2", Descricao: "Produto de teste", Saldo: 2}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_2")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 5}}
	err := reservasStore.Reservar("TEST_CHAVE_2", itens)
	if err == nil {
		t.Fatal("esperava erro de saldo insuficiente, mas Reservar nao retornou erro")
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 2 {
		t.Errorf("saldo nao deveria ter mudado, esperado 2, obtido %d", atualizado.Saldo)
	}
}
func TestConfirmar_MarcaComoConfirmada(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_CONFIRMAR_1", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_CONFIRMAR")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 3}}
	if err := reservasStore.Reservar("TEST_CHAVE_CONFIRMAR", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	if err := reservasStore.Confirmar("TEST_CHAVE_CONFIRMAR"); err != nil {
		t.Fatalf("Confirmar retornou erro inesperado: %v", err)
	}

	// confirmar de novo deve falhar, porque a reserva ja nao esta mais pendente
	if err := reservasStore.Confirmar("TEST_CHAVE_CONFIRMAR"); err == nil {
		t.Error("esperava erro ao confirmar uma reserva ja confirmada, mas nao retornou erro")
	}
}

func TestCancelar_DevolveSaldo(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_CANCELAR_1", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_CANCELAR")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 4}}
	if err := reservasStore.Reservar("TEST_CHAVE_CANCELAR", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	if err := reservasStore.Cancelar("TEST_CHAVE_CANCELAR"); err != nil {
		t.Fatalf("Cancelar retornou erro inesperado: %v", err)
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 10 {
		t.Errorf("saldo esperado 10 (devolvido), obtido %d", atualizado.Saldo)
	}
}
