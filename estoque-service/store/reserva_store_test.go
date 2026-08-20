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

func TestReservar_NaoMexeNoSaldo(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_RESERVAR_1", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_1")

	// Reservar so registra a intencao, nao verifica nem desconta saldo -
	// por isso funciona mesmo pedindo mais do que o produto tem.
	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 999}}
	if err := reservasStore.Reservar("TEST_CHAVE_1", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 10 {
		t.Errorf("saldo nao deveria ter mudado, esperado 10, obtido %d", atualizado.Saldo)
	}
}

func TestConfirmar_DescontaSaldo(t *testing.T) {
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

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 7 {
		t.Errorf("saldo esperado 7, obtido %d", atualizado.Saldo)
	}

	// confirmar de novo deve falhar, porque a reserva ja nao esta mais pendente
	if err := reservasStore.Confirmar("TEST_CHAVE_CONFIRMAR"); err == nil {
		t.Error("esperava erro ao confirmar uma reserva ja confirmada, mas nao retornou erro")
	}
}

func TestConfirmar_SaldoInsuficiente(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	// simula a disputa: o produto so tem 2 unidades, mas a reserva pede 5
	// (ex: outra nota ja confirmou antes e consumiu o saldo)
	produto := &models.Produto{Codigo: "TEST_CONFIRMAR_2", Descricao: "Produto de teste", Saldo: 2}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_DISPUTA")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 5}}
	if err := reservasStore.Reservar("TEST_CHAVE_DISPUTA", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	if err := reservasStore.Confirmar("TEST_CHAVE_DISPUTA"); err == nil {
		t.Fatal("esperava erro de saldo insuficiente, mas Confirmar nao retornou erro")
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 2 {
		t.Errorf("saldo nao deveria ter mudado, esperado 2, obtido %d", atualizado.Saldo)
	}

	// a reserva continua pendente - pode tentar confirmar de novo depois
	somas, err := reservasStore.SomaPendentesPorProduto()
	if err != nil {
		t.Fatalf("SomaPendentesPorProduto retornou erro inesperado: %v", err)
	}
	if somas[produto.Codigo] != 5 {
		t.Errorf("reserva deveria continuar pendente com 5 unidades, obtido %d", somas[produto.Codigo])
	}
}

func TestConfirmar_ReservaExpirada(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_CONFIRMAR_EXPIRADA", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_EXPIRADA")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 3}}
	if err := reservasStore.Reservar("TEST_CHAVE_EXPIRADA", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	// simula uma reserva criada ha mais tempo que o TTL permite (7 dias)
	if _, err := testDB.Exec(
		`UPDATE reservas SET criado_em = NOW() - $1::interval WHERE chave = $2`,
		"8 days", "TEST_CHAVE_EXPIRADA",
	); err != nil {
		t.Fatalf("falha ao forcar reserva expirada no teste: %v", err)
	}

	if err := reservasStore.Confirmar("TEST_CHAVE_EXPIRADA"); err == nil {
		t.Fatal("esperava erro de reserva expirada, mas Confirmar nao retornou erro")
	}

	// nao deve ter descontado saldo - a confirmacao foi recusada antes de checar produtos
	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 10 {
		t.Errorf("saldo nao deveria ter mudado, esperado 10, obtido %d", atualizado.Saldo)
	}

	// a reserva deve ter sido cancelada automaticamente, entao nao conta mais como pendente
	somas, err := reservasStore.SomaPendentesPorProduto()
	if err != nil {
		t.Fatalf("SomaPendentesPorProduto retornou erro inesperado: %v", err)
	}
	if somas[produto.Codigo] != 0 {
		t.Errorf("reserva expirada deveria ter sido cancelada, nao deveria contar como pendente, obtido %d", somas[produto.Codigo])
	}

	// tentar confirmar de novo deve falhar porque a reserva ja nao esta mais pendente
	if err := reservasStore.Confirmar("TEST_CHAVE_EXPIRADA"); err == nil {
		t.Error("esperava erro ao confirmar reserva ja cancelada por expiracao, mas nao retornou erro")
	}
}

func TestCancelar_NaoMexeNoSaldo(t *testing.T) {
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

	// Reservar nunca descontou nada, entao Cancelar nao tem nada pra devolver
	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 10 {
		t.Errorf("saldo esperado 10 (nunca foi descontado), obtido %d", atualizado.Saldo)
	}

	// cancelar de novo deve ter sucesso (idempotente) - a reserva ja esta
	// cancelada, chamar de novo nao deveria ser erro (suporta retry seguro)
	if err := reservasStore.Cancelar("TEST_CHAVE_CANCELAR"); err != nil {
		t.Errorf("Cancelar deveria ser idempotente (reserva ja cancelada), mas retornou erro: %v", err)
	}
}

func TestCancelar_ReservaConfirmada_Falha(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_CANCELAR_CONFIRMADA", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave = $1`, "TEST_CHAVE_CANCELAR_CONFIRMADA")

	itens := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 4}}
	if err := reservasStore.Reservar("TEST_CHAVE_CANCELAR_CONFIRMADA", itens); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}
	if err := reservasStore.Confirmar("TEST_CHAVE_CANCELAR_CONFIRMADA"); err != nil {
		t.Fatalf("Confirmar retornou erro inesperado: %v", err)
	}

	// uma reserva ja confirmada (saldo ja debitado de verdade) nao pode ser cancelada
	if err := reservasStore.Cancelar("TEST_CHAVE_CANCELAR_CONFIRMADA"); err == nil {
		t.Error("esperava erro ao cancelar uma reserva ja confirmada, mas nao retornou erro")
	}

	atualizado, err := produtosStore.GetByCodigo(produto.Codigo)
	if err != nil {
		t.Fatalf("falha ao buscar produto atualizado: %v", err)
	}
	if atualizado.Saldo != 6 {
		t.Errorf("saldo nao deveria ter mudado apos a tentativa de cancelamento, esperado 6, obtido %d", atualizado.Saldo)
	}
}

func TestSomaPendentesPorProduto(t *testing.T) {
	produtosStore := NewProdutosStore(testDB)
	reservasStore := NewReservasStore(testDB)

	produto := &models.Produto{Codigo: "TEST_PENDENTE_1", Descricao: "Produto de teste", Saldo: 10}
	if err := produtosStore.Create(produto); err != nil {
		t.Fatalf("falha ao criar produto de teste: %v", err)
	}
	defer testDB.Exec(`DELETE FROM produtos WHERE codigo = $1`, produto.Codigo)
	defer testDB.Exec(`DELETE FROM reservas WHERE chave IN ($1, $2)`, "TEST_CHAVE_PENDENTE", "TEST_CHAVE_CONFIRMADA")

	// reserva que fica pendente (nao confirma nem cancela)
	itensPendente := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 3}}
	if err := reservasStore.Reservar("TEST_CHAVE_PENDENTE", itensPendente); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}

	// reserva confirmada nao deveria contar como pendente
	itensConfirmada := []models.ItemReserva{{ProdutoCodigo: produto.Codigo, Quantidade: 2}}
	if err := reservasStore.Reservar("TEST_CHAVE_CONFIRMADA", itensConfirmada); err != nil {
		t.Fatalf("Reservar retornou erro inesperado: %v", err)
	}
	if err := reservasStore.Confirmar("TEST_CHAVE_CONFIRMADA"); err != nil {
		t.Fatalf("Confirmar retornou erro inesperado: %v", err)
	}

	somas, err := reservasStore.SomaPendentesPorProduto()
	if err != nil {
		t.Fatalf("SomaPendentesPorProduto retornou erro inesperado: %v", err)
	}
	if somas[produto.Codigo] != 3 {
		t.Errorf("soma pendente esperada 3 (so a reserva nao confirmada), obtida %d", somas[produto.Codigo])
	}
}
