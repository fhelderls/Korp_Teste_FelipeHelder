package store

import (
	"database/sql"

	"korp-teste/estoque-service/models"
)

type ProdutosStore struct {
	db *sql.DB
}

func NewProdutosStore(db *sql.DB) *ProdutosStore {
	return &ProdutosStore{db: db}
}

// Create insere um novo produto
func (s *ProdutosStore) Create(produto *models.Produto) error {
	_, err := s.db.Exec(`
		INSERT INTO produtos (codigo, descricao, saldo)
		VALUES ($1, $2, $3)`, produto.Codigo, produto.Descricao, produto.Saldo)
	return err
}

// GetAll retorna todos os produtos cadastrados no banco de dados
func (s *ProdutosStore) GetAll() ([]models.Produto, error) {
	rows, err := s.db.Query(`SELECT codigo, descricao, saldo FROM produtos ORDER BY codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	produtos := []models.Produto{}

	for rows.Next() {
		var produto models.Produto
		if err := rows.Scan(&produto.Codigo, &produto.Descricao, &produto.Saldo); err != nil {
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
		`SELECT codigo, descricao, saldo FROM produtos WHERE codigo = $1`,
		codigo,
	).Scan(&p.Codigo, &p.Descricao, &p.Saldo)
	return p, err
}
