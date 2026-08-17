package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"korp-teste/faturamento-service/models"
)

type NotasStore struct {
	db *sql.DB
}

func NewNotasStore(db *sql.DB) *NotasStore {
	return &NotasStore{db: db}

}

// Create insere uma nova nota fiscal com numero sequencial gerado
// automaticamente (NF-001, NF-002...) e status 'Aberta'. Preenche
// nota.Chave com o numero gerado. Nao mexe em estoque nenhum - so grava
// o cadastro, conforme o edital pede (criar e imprimir sao acoes
// separadas).
func (s *NotasStore) Create(nota *models.NotaFiscal) error {
	var numero int
	if err := s.db.QueryRow(`SELECT nextval('notas_numero_seq')`).Scan(&numero); err != nil {
		return fmt.Errorf("falha ao gerar numero da nota: %w", err)
	}
	nota.Chave = fmt.Sprintf("NF-%03d", numero)

	itensJSON, err := json.Marshal(nota.Itens)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO notas (chave, cliente, itens, status) VALUES ($1,$2,$3,'Aberta')`,
		nota.Chave, nota.Cliente, string(itensJSON),
	)
	return err

}

// AtualizarStatus muda o status de uma nota fiscal (ex: para "Fechada").
func (s *NotasStore) AtualizarStatus(chave, status string) error {
	_, err := s.db.Exec(
		`UPDATE notas SET status = $1 WHERE chave = $2`,
		status, chave,
	)
	return err
}

// GetAll retona todas as notas fiscais cadastradas.
func (s *NotasStore) GetAll() ([]models.NotaFiscal, error) {
	rows, err := s.db.Query(`SELECT chave, cliente, itens, status FROM notas ORDER BY criado_em DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notas := []models.NotaFiscal{}

	for rows.Next() {
		var nota models.NotaFiscal
		var itensJson string
		if err := rows.Scan(&nota.Chave, &nota.Cliente, &itensJson, &nota.Status); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(itensJson), &nota.Itens); err != nil {
			return nil, err
		}
		notas = append(notas, nota)
	}
	return notas, rows.Err()
}

// GetByChave busca uma nota fiscal pela chave. Retorna sql.ErrNoRows se nao existir.
func (s *NotasStore) GetByChave(chave string) (models.NotaFiscal, error) {
	var nota models.NotaFiscal
	var itensJSON string
	err := s.db.QueryRow(
		`SELECT chave, cliente, itens, status FROM notas WHERE chave = $1`,
		chave,
	).Scan(&nota.Chave, &nota.Cliente, &itensJSON, &nota.Status)
	if err != nil {
		return nota, err
	}
	if err := json.Unmarshal([]byte(itensJSON), &nota.Itens); err != nil {
		return nota, err
	}
	return nota, nil
}
