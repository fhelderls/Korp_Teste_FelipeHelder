package store

import (
	"database/sql"
	"os"
	"testing"

	"korp-teste/faturamento-service/db"
	"korp-teste/faturamento-service/models"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgres://korp:korp@localhost:5435/faturamento?sslmode=disable"
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

func TestCreate_GetByChave(t *testing.T) {
	store := NewNotasStore(testDB)

	nota := &models.NotaFiscal{
		Cliente: "Cliente Teste",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 2, Descricao: "Produto Teste", PrecoUnitario: 9.9}},
	}
	if err := store.Create(nota); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	// Create preenche nota.Chave com o numero sequencial gerado (ex: NF-001)
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, nota.Chave)

	if nota.Chave == "" {
		t.Fatal("Create deveria ter preenchido nota.Chave com o numero gerado")
	}

	encontrada, err := store.GetByChave(nota.Chave)
	if err != nil {
		t.Fatalf("GetByChave retornou erro inesperado: %v", err)
	}
	if encontrada.Status != "Aberta" {
		t.Errorf("status esperado 'Aberta', obtido %q", encontrada.Status)
	}
	if len(encontrada.Itens) != 1 || encontrada.Itens[0].ProdutoCodigo != "P001" {
		t.Errorf("itens nao vieram como esperado: %+v", encontrada.Itens)
	}
	// descricao e preco sao um retrato do produto gravado com a nota - tem
	// que sobreviver ao round-trip de serializacao/persistencia
	if encontrada.Itens[0].Descricao != "Produto Teste" || encontrada.Itens[0].PrecoUnitario != 9.9 {
		t.Errorf("descricao/preco do item nao foram preservados: %+v", encontrada.Itens[0])
	}
	if encontrada.DataAbertura.IsZero() {
		t.Error("DataAbertura deveria ter sido preenchida na criacao")
	}
	if encontrada.DataEmissao != nil {
		t.Error("DataEmissao deveria ser nula antes da emissao")
	}
}

func TestMarcarEmitida(t *testing.T) {
	store := NewNotasStore(testDB)

	nota := &models.NotaFiscal{
		Cliente: "Cliente Teste",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 1}},
	}
	if err := store.Create(nota); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, nota.Chave)

	dataEmissao, err := store.MarcarEmitida(nota.Chave)
	if err != nil {
		t.Fatalf("MarcarEmitida retornou erro inesperado: %v", err)
	}
	if dataEmissao.IsZero() {
		t.Error("MarcarEmitida deveria ter retornado a data de emissao")
	}

	encontrada, err := store.GetByChave(nota.Chave)
	if err != nil {
		t.Fatalf("GetByChave retornou erro inesperado: %v", err)
	}
	if encontrada.Status != "Fechada" {
		t.Errorf("status esperado 'Fechada', obtido %q", encontrada.Status)
	}
	if encontrada.DataEmissao == nil {
		t.Error("DataEmissao deveria ter sido preenchida apos a emissao")
	}
}

func TestListarAbertasVencidas(t *testing.T) {
	store := NewNotasStore(testDB)

	notaVencida := &models.NotaFiscal{
		Cliente: "Cliente Vencido",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 1}},
	}
	if err := store.Create(notaVencida); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, notaVencida.Chave)

	notaRecente := &models.NotaFiscal{
		Cliente: "Cliente Recente",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 1}},
	}
	if err := store.Create(notaRecente); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, notaRecente.Chave)

	// forca a primeira nota a parecer criada ha mais tempo que o TTL (7 dias)
	if _, err := testDB.Exec(
		`UPDATE notas SET criado_em = NOW() - $1::interval WHERE chave = $2`,
		"8 days", notaVencida.Chave,
	); err != nil {
		t.Fatalf("falha ao forcar nota vencida no teste: %v", err)
	}

	chaves, err := store.ListarAbertasVencidas()
	if err != nil {
		t.Fatalf("ListarAbertasVencidas retornou erro inesperado: %v", err)
	}

	achouVencida := false
	for _, c := range chaves {
		if c == notaVencida.Chave {
			achouVencida = true
		}
		if c == notaRecente.Chave {
			t.Errorf("nota recente %s nao deveria aparecer como vencida", notaRecente.Chave)
		}
	}
	if !achouVencida {
		t.Errorf("nota vencida %s deveria ter aparecido na lista, obtido %v", notaVencida.Chave, chaves)
	}

	// ListarAbertasVencidas so consulta - nao deve ter mudado o status de ninguem
	encontrada, err := store.GetByChave(notaVencida.Chave)
	if err != nil {
		t.Fatalf("GetByChave retornou erro inesperado: %v", err)
	}
	if encontrada.Status != "Aberta" {
		t.Errorf("ListarAbertasVencidas nao deveria mudar o status, esperado 'Aberta', obtido %q", encontrada.Status)
	}
}

func TestMarcarCancelada(t *testing.T) {
	store := NewNotasStore(testDB)

	nota := &models.NotaFiscal{
		Cliente: "Cliente Teste",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 1}},
	}
	if err := store.Create(nota); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, nota.Chave)

	if err := store.MarcarCancelada(nota.Chave); err != nil {
		t.Fatalf("MarcarCancelada retornou erro inesperado: %v", err)
	}

	encontrada, err := store.GetByChave(nota.Chave)
	if err != nil {
		t.Fatalf("GetByChave retornou erro inesperado: %v", err)
	}
	if encontrada.Status != "Cancelada" {
		t.Errorf("status esperado 'Cancelada', obtido %q", encontrada.Status)
	}

	// nao deve mexer numa nota ja 'Fechada' (MarcarCancelada so vale para 'Aberta')
	notaFechada := &models.NotaFiscal{
		Cliente: "Cliente Fechado",
		Itens:   []models.ItemNota{{ProdutoCodigo: "P001", Quantidade: 1}},
	}
	if err := store.Create(notaFechada); err != nil {
		t.Fatalf("Create retornou erro inesperado: %v", err)
	}
	defer testDB.Exec(`DELETE FROM notas WHERE chave = $1`, notaFechada.Chave)
	if _, err := store.MarcarEmitida(notaFechada.Chave); err != nil {
		t.Fatalf("MarcarEmitida retornou erro inesperado: %v", err)
	}

	if err := store.MarcarCancelada(notaFechada.Chave); err != nil {
		t.Fatalf("MarcarCancelada nao deveria retornar erro (so nao afeta linha nenhuma): %v", err)
	}
	aindaFechada, err := store.GetByChave(notaFechada.Chave)
	if err != nil {
		t.Fatalf("GetByChave retornou erro inesperado: %v", err)
	}
	if aindaFechada.Status != "Fechada" {
		t.Errorf("nota 'Fechada' nao deveria ter sido alterada por MarcarCancelada, obtido %q", aindaFechada.Status)
	}
}

func TestGetByChave_NaoEncontrada(t *testing.T) {
	store := NewNotasStore(testDB)

	_, err := store.GetByChave("CHAVE_QUE_NAO_EXISTE")
	if err != sql.ErrNoRows {
		t.Errorf("esperava sql.ErrNoRows, obtido %v", err)
	}
}
