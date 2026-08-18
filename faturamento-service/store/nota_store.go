package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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
// nota.Chave e nota.DataAbertura com os valores gerados pelo banco. Nao
// mexe em estoque nenhum - so grava o cadastro, conforme o edital pede
// (criar e imprimir sao acoes separadas).
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
	err = s.db.QueryRow(
		`INSERT INTO notas (chave, cliente, itens, status) VALUES ($1,$2,$3,'Aberta') RETURNING criado_em`,
		nota.Chave, nota.Cliente, string(itensJSON),
	).Scan(&nota.DataAbertura)
	if err != nil {
		return err
	}
	nota.Status = "Aberta"
	return nil

}

// MarcarEmitida muda o status de uma nota fiscal para 'Fechada' e grava a
// data/hora de emissao (chamado quando a impressao processa a nota com
// sucesso). Retorna a data de emissao gravada.
func (s *NotasStore) MarcarEmitida(chave string) (time.Time, error) {
	var dataEmissao time.Time
	err := s.db.QueryRow(
		`UPDATE notas SET status = 'Fechada', data_emissao = NOW() WHERE chave = $1 RETURNING data_emissao`,
		chave,
	).Scan(&dataEmissao)
	return dataEmissao, err
}

// GetAll retona todas as notas fiscais cadastradas.
func (s *NotasStore) GetAll() ([]models.NotaFiscal, error) {
	rows, err := s.db.Query(`SELECT chave, cliente, itens, status, criado_em, data_emissao FROM notas ORDER BY criado_em DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notas := []models.NotaFiscal{}

	for rows.Next() {
		var nota models.NotaFiscal
		var itensJson string
		var dataEmissao sql.NullTime
		if err := rows.Scan(&nota.Chave, &nota.Cliente, &itensJson, &nota.Status, &nota.DataAbertura, &dataEmissao); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(itensJson), &nota.Itens); err != nil {
			return nil, err
		}
		if dataEmissao.Valid {
			nota.DataEmissao = &dataEmissao.Time
		}
		notas = append(notas, nota)
	}
	return notas, rows.Err()
}

// GetByChave busca uma nota fiscal pela chave. Retorna sql.ErrNoRows se nao existir.
func (s *NotasStore) GetByChave(chave string) (models.NotaFiscal, error) {
	var nota models.NotaFiscal
	var itensJSON string
	var dataEmissao sql.NullTime
	err := s.db.QueryRow(
		`SELECT chave, cliente, itens, status, criado_em, data_emissao FROM notas WHERE chave = $1`,
		chave,
	).Scan(&nota.Chave, &nota.Cliente, &itensJSON, &nota.Status, &nota.DataAbertura, &dataEmissao)
	if err != nil {
		return nota, err
	}
	if err := json.Unmarshal([]byte(itensJSON), &nota.Itens); err != nil {
		return nota, err
	}
	if dataEmissao.Valid {
		nota.DataEmissao = &dataEmissao.Time
	}
	return nota, nil
}
