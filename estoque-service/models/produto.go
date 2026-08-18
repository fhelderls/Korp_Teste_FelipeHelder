package models

//produto representa um item de estoque e a quantidade disponível
type Produto struct {
	Codigo    string  `json:"codigo"`
	Descricao string  `json:"descricao"`
	Saldo     int     `json:"saldo"`
	Preco     float64 `json:"preco"`
	// SaldoReservado e SaldoDisponivel sao calculados na hora (nao ficam
	// gravados na tabela produtos): SaldoReservado e a soma das reservas
	// ainda 'pendente' (nem confirmadas, nem canceladas) desse produto, e
	// SaldoDisponivel e Saldo - SaldoReservado. Preenchidos pelo handler,
	// nao pelo store.
	SaldoReservado  int `json:"saldo_reservado"`
	SaldoDisponivel int `json:"saldo_disponivel"`
}
