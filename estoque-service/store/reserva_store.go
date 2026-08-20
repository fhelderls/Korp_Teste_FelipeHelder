package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"korp-teste/estoque-service/models"
)

// TTLReserva e o tempo maximo que uma reserva pode ficar 'pendente' antes de
// nao poder mais ser confirmada. Existe pra fechar uma brecha de negocio: como
// o faturamento-service trava a descricao e o preco do produto no momento em
// que a nota e criada (Reservar), mas o saldo so e verificado na confirmacao,
// uma nota 'Aberta' sem limite de tempo vira um jeito de segurar um preco
// antigo indefinidamente (ex: criar notas durante uma promocao relampago e so
// confirmar depois que o preco normal voltou). Passado o TTL, a proxima
// tentativa de confirmar cancela a reserva automaticamente em vez de aceitar.
// 7 dias segue a mesma logica de um orcamento comercial com prazo de validade
// (ex: "orcamento valido por 7 dias"), em vez de um valor arbitrario curto.
const TTLReserva = 7 * 24 * time.Hour

type ReservasStore struct {
	db *sql.DB
}

func NewReservasStore(db *sql.DB) *ReservasStore {
	return &ReservasStore{db: db}
}

// SomaPendentesPorProduto soma a quantidade reservada (em reservas ainda
// 'pendente' - nem confirmadas, nem canceladas) de cada produto. Usado
// pra calcular o saldo disponivel (saldo total menos o que esta retido
// em notas ainda abertas). Como Reservar nao verifica saldo, essa soma
// pode passar do saldo total quando varias notas abertas disputam o
// mesmo produto - o saldo disponivel fica negativo nesse caso, sinalizando
// a disputa.
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

// Reservar registra a intencao de uso de estoque de uma nota fiscal
// assim que ela e criada. Nao verifica nem desconta saldo - so guarda os
// itens com status 'pendente'. A disputa por estoque entre notas
// concorrentes so e resolvida depois, na confirmacao. Se a chave ja
// existir, retorna erro (idempotencia: o faturamento-service pode repetir
// a chamada com seguranca).
func (s *ReservasStore) Reservar(chave string, itens []models.ItemReserva) error {
	itensJSON, err := json.Marshal(itens)
	if err != nil {
		return fmt.Errorf("falha ao serializar itens para JSON: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO reservas (chave, status, itens) VALUES ($1, 'pendente', $2)`,
		chave, string(itensJSON),
	)
	if err != nil {
		return fmt.Errorf("falha ao inserir reserva: %w", err)
	}
	return nil
}

// Confirmar e onde a disputa por estoque de verdade acontece: verifica se
// ha saldo suficiente para cada item da reserva e, se tiver, desconta o
// estoque e marca a reserva como confirmada - tudo numa unica transacao,
// com SELECT ... FOR UPDATE travando a linha de cada produto. Se duas
// notas abertas reservaram mais do que o estoque tem, a primeira a
// confirmar leva o saldo; a outra falha aqui com "estoque insuficiente" e
// continua 'pendente' (a nota fiscal continua 'Aberta', pode tentar de
// novo depois se o saldo for reposto).
func (s *ReservasStore) Confirmar(chave string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("falha ao iniciar transacao: %w", err)
	}
	defer tx.Rollback()

	var status string
	var itensJSON string
	var criadoEm time.Time
	err = tx.QueryRow(
		`SELECT status, itens, criado_em FROM reservas WHERE chave = $1 FOR UPDATE`, chave,
	).Scan(&status, &itensJSON, &criadoEm)
	if err != nil {
		return fmt.Errorf("reserva %s: %w", chave, err)
	}
	if status != "pendente" {
		return fmt.Errorf("reserva %s nao esta pendente, status atual %s", chave, status)
	}

	if time.Since(criadoEm) > TTLReserva {
		if _, err := tx.Exec(`UPDATE reservas SET status = 'cancelada' WHERE chave = $1`, chave); err != nil {
			return fmt.Errorf("falha ao cancelar reserva expirada %s: %w", chave, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("falha ao registrar cancelamento da reserva expirada %s: %w", chave, err)
		}
		return fmt.Errorf("reserva %s expirou (criada ha mais de %s) e foi cancelada automaticamente", chave, TTLReserva)
	}

	var itens []models.ItemReserva
	if err := json.Unmarshal([]byte(itensJSON), &itens); err != nil {
		return fmt.Errorf("falha ao desserializar itens da reserva %s: %w", chave, err)
	}

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

	_, err = tx.Exec(`UPDATE reservas SET status = 'confirmada' WHERE chave = $1`, chave)
	if err != nil {
		return fmt.Errorf("falha ao confirmar reserva: %w", err)
	}

	return tx.Commit()
}

// Cancelar marca uma reserva como cancelada, liberando a reserva de estoque
// (sem devolver saldo, ja que Reservar nunca descontou nada). E idempotente:
// cancelar uma reserva que ja esta 'cancelada' e sucesso (no-op), pra
// suportar chamadas repetidas com seguranca - por exemplo, o
// faturamento-service tentando de novo depois de uma falha de rede, ou uma
// reserva que ja tinha expirado sozinha pelo TTL embutido no Confirmar (ver
// TTLReserva). So continua sendo erro cancelar uma reserva que nao existe ou
// que ja foi 'confirmada' - essa e definitiva, o saldo ja foi debitado de
// verdade e nao pode ser desfeita por aqui.
func (s *ReservasStore) Cancelar(chave string) error {
	res, err := s.db.Exec(
		`UPDATE reservas SET status = 'cancelada' WHERE chave = $1 AND status IN ('pendente', 'cancelada')`,
		chave,
	)
	if err != nil {
		return fmt.Errorf("falha ao cancelar reserva: %w", err)
	}
	linhas, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("falha ao verificar cancelamento de reserva %s: %w", chave, err)
	}
	if linhas == 0 {
		return fmt.Errorf("reserva %s nao encontrada ou ja confirmada", chave)
	}
	return nil
}
