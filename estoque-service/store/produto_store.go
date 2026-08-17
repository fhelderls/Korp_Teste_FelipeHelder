package store

import (
	"database/sql"
	"fmt"

	"korp-teste/estoque-service/models"
)

type ProdutosStore struct {
	db *sql.DB
}

func NewProdutosStore(db *sql.DB) *ProdutosStore {
	return &ProdutosStore{db: db}
}

// Create insere um novo produto com codigo sequencial gerado automaticamente
// (PROD-001, PROD-002...). Preenche produto.Codigo com o codigo gerado.
func (s *ProdutosStore) Create(produto *models.Produto) error {
	var numero int
	if err := s.db.QueryRow(`SELECT nextval('produtos_codigo_seq')`).Scan(&numero); err != nil {
		return fmt.Errorf("falha ao gerar codigo do produto: %w", err)
	}
	produto.Codigo = fmt.Sprintf("PROD-%03d", numero)

	_, err := s.db.Exec(`
		INSERT INTO produtos (codigo, descricao, saldo, preco)
		VALUES ($1, $2, $3, $4)`, produto.Codigo, produto.Descricao, produto.Saldo, produto.Preco)
	return err
}

// GetAll retorna todos os produtos cadastrados no banco de dados
func (s *ProdutosStore) GetAll() ([]models.Produto, error) {
	rows, err := s.db.Query(`SELECT codigo, descricao, saldo, preco FROM produtos ORDER BY codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	produtos := []models.Produto{}

	for rows.Next() {
		var produto models.Produto
		if err := rows.Scan(&produto.Codigo, &produto.Descricao, &produto.Saldo, &produto.Preco); err != nil {
			return nil, err
		}
		produtos = append(produtos, produto)
	}
	return produtos, rows.Err()

}

// GetByCodigo busca um produto pelo codigo. Retorna sql.ErrNoRows se nao existir.
func (s *ProdutosStore) GetByCodigo(codigo string) (models.Produto, error) {
	var p models.Produto
	err := s.db.QueryRow(
		`SELECT codigo, descricao, saldo, preco FROM produtos WHERE codigo = $1`,
		codigo,
	).Scan(&p.Codigo, &p.Descricao, &p.Saldo, &p.Preco)
	return p, err
}

// Delete remove um produto pelo codigo.
func (s *ProdutosStore) Delete(codigo string) error {
	_, err := s.db.Exec(`DELETE FROM produtos WHERE codigo = $1`, codigo)
	return err
}
