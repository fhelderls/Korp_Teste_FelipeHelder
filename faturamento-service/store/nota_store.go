package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"korp-teste/faturamento-service/models"
)

// TTLReserva e o prazo de validade de uma nota 'Aberta' - depois disso, ela
// e candidata a expiracao automatica (ver ListarAbertasVencidas). Espelha o
// TTLReserva do estoque-service: os dois servicos tem bancos isolados e nao
// compartilham codigo, entao o mesmo valor de negocio (validade de 7 dias,
// no estilo de um orcamento comercial) precisa ficar sincronizado manualmente
// nos dois lados.
const TTLReserva = 7 * 24 * time.Hour

type NotasStore struct {
	db *sql.DB
}

func NewNotasStore(db *sql.DB) *NotasStore {
	return &NotasStore{db: db}

}

// Create insere uma nova nota fiscal com numero sequencial gerado
// automaticamente (NF-001, NF-002...) e status 'Aberta'. Preenche
// nota.Chave e nota.DataAbertura com os valores gerados pelo banco.
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

// Delete remove uma nota fiscal pela chave. Usado como compensacao quando
// a nota e criada mas a reserva de estoque correspondente falha - desfaz
// o cadastro em vez de deixar uma nota 'Aberta' sem estoque reservado.
func (s *NotasStore) Delete(chave string) error {
	_, err := s.db.Exec(`DELETE FROM notas WHERE chave = $1`, chave)
	return err
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

// ListarAbertasVencidas retorna as chaves das notas 'Aberta' cuja reserva de
// estoque ja passou do prazo de validade (TTLReserva) - candidatas a serem
// canceladas automaticamente. So consulta, nao muda nada: a nota so vira
// 'Cancelada' de fato depois que a reserva correspondente for liberada com
// sucesso no estoque-service (ver MarcarCancelada e o handler que orquestra
// os dois passos).
func (s *NotasStore) ListarAbertasVencidas() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT chave FROM notas WHERE status = 'Aberta' AND criado_em < NOW() - $1::interval`,
		fmt.Sprintf("%d seconds", int64(TTLReserva.Seconds())),
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar notas vencidas: %w", err)
	}
	defer rows.Close()

	var chaves []string
	for rows.Next() {
		var chave string
		if err := rows.Scan(&chave); err != nil {
			return nil, err
		}
		chaves = append(chaves, chave)
	}
	return chaves, rows.Err()
}

// MarcarCancelada muda o status de uma nota fiscal 'Aberta' para 'Cancelada'
// (sem remove-la, ao contrario do cancelamento manual pelo usuario, que
// exclui a nota). Usado so na expiracao automatica por TTL: mantem o
// registro historico de que a nota existiu e expirou, em vez de apagar.
func (s *NotasStore) MarcarCancelada(chave string) error {
	_, err := s.db.Exec(`UPDATE notas SET status = 'Cancelada' WHERE chave = $1 AND status = 'Aberta'`, chave)
	return err
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
