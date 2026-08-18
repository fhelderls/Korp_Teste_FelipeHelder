package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"korp-teste/estoque-service/models"
)

type ReservasStore struct {
	db *sql.DB
}

func NewReservasStore(db *sql.DB) *ReservasStore {
	return &ReservasStore{db: db}
}

// SomaPendentesPorProduto soma a quantidade reservada (em reservas ainda
// 'pendente' - nem confirmadas, nem canceladas) de cada produto. Usado
// pra calcular o saldo disponivel (saldo total menos o que esta retido
// em reservas em andamento).
func (s *ReservasStore) SomaPendentesPorProduto() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT itens FROM reservas WHERE status = 'pendente'`)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar reservas pendentes: %w", err)
	}
	defer rows.Close()

	somas := map[string]int{}
	for rows.Next() {
		var itensJSON string
		if err := rows.Scan(&itensJSON); err != nil {
			return nil, err
		}
		var itens []models.ItemReserva
		if err := json.Unmarshal([]byte(itensJSON), &itens); err != nil {
			return nil, fmt.Errorf("falha ao desserializar itens da reserva: %w", err)
		}
		for _, item := range itens {
			somas[item.ProdutoCodigo] += item.Quantidade
		}
	}
	return somas, rows.Err()
}

// Reservar valida o saldo de cada item e desconta o estoque, tudo dentro
// de uma unica transacao. Se a chave ja existir, retorna erro (idempotencia:
// o faturamento-service pode repetir a chamada com seguranca).

func (s *ReservasStore) Reservar(chave string, itens []models.ItemReserva) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("falha ao iniciar transacao: %w", err)
	}
	defer tx.Rollback()

	for _, item := range itens {
		var saldoAtual int
		err := tx.QueryRow(
			`SELECT saldo FROM produtos WHERE codigo = $1 FOR UPDATE`,
			item.ProdutoCodigo,
		).Scan(&saldoAtual)
		if err != nil {
			return fmt.Errorf("produto %s: %w", item.ProdutoCodigo, err)
		}

		if saldoAtual < item.Quantidade {
			return fmt.Errorf("estoque insuficiente para o produto %s", item.ProdutoCodigo)
		}

		_, err = tx.Exec(
			`UPDATE produtos SET saldo = saldo - $1 WHERE codigo = $2`,
			item.Quantidade, item.ProdutoCodigo,
		)
		if err != nil {
			return fmt.Errorf("falha ao atualizar saldo do produto %s: %w", item.ProdutoCodigo, err)
		}
	}

	itensJSON, err := json.Marshal(itens)
	if err != nil {
		return fmt.Errorf("falha a serializar itens para JSON: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO reservas (chave, status, itens) VALUES ($1, 'pendente', $2)`,
		chave, string(itensJSON),
	)
	if err != nil {
		return fmt.Errorf("falha ao inserir reserva: %w", err)
	}

	return tx.Commit()
}

// Confirmar marca uma reserva como confirmada, chamando quanndo a nota fiscal foi emitida com sucesso.
//O saldo ja foi descontado em Reservar, e continua descontado, so o status muda.

func (s *ReservasStore) Confirmar(chave string) error {
	res, err := s.db.Exec(
		`UPDATE reservas SET status = 'confirmada' WHERE chave = $1 AND status = 'pendente'`,

		chave,
	)
	if err != nil {
		return fmt.Errorf("falha ao confirmar reserva: %w", err)
	}
	linhas, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("Falha ao verificar confirmacao de reserva %s: %w", chave, err)
	}
	if linhas == 0 {
		return fmt.Errorf("reserva %s nao encontrada ou nao esta pendente", chave)

	}
	return nil
}

// Cancelar devolve o saldo reservado e marca a reserva como cancelada.
// e a acao de compensacao do padrao Saga: usada quando uma etapa posterior (emissao de nota fiscal)
// falha depois que o estoque ja tinha sido retido

func (s *ReservasStore) Cancelar(chave string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("Falha ao iniciar transação: %w", err)

	}
	defer tx.Rollback()

	var itensJSON string
	var status string
	err = tx.QueryRow(
		`SELECT status, itens FROM reservas WHERE chave = $1 FOR UPDATE`, chave,
	).Scan(&status, &itensJSON)
	if err != nil {
		return fmt.Errorf("reserva %s: %w", chave, err)

	}
	if status != "pendente" {
		return fmt.Errorf("reserva %s nao esta pendente, status atual %s", chave, status)

	}

	var itens []models.ItemReserva

	if err := json.Unmarshal([]byte(itensJSON), &itens); err != nil {
		return fmt.Errorf("Falha ao desserializar itens da reserva %s: %w", chave, err)

	}

	for _, item := range itens {
		_, err = tx.Exec(
			`UPDATE produtos SET saldo = saldo + $1 WHERE codigo = $2`,
			item.Quantidade, item.ProdutoCodigo,
		)
		if err != nil {
			return fmt.Errorf("falha ao devolver saldo do produto %s, %w", item.ProdutoCodigo, err)

		}

	}
	_, err = tx.Exec(
		`UPDATE reservas SET status = 'cancelada' WHERE chave = $1`,
		chave,
	)
	if err != nil {
		return fmt.Errorf("Falha ao marcar reserva %s como cancelada: %w", chave, err)
	}
	return tx.Commit()
}
