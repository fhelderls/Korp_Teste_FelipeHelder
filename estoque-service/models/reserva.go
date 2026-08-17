package models

// ItemReserva e um produto e a quantidade dentro de uma reserva
type ItemReserva struct {
	ProdutoCodigo string `json:"produto_codigo"`
	Quantidade    int    `json:"quantidade"`
}

// Reserva representa uma retenção de estoque vinculada a uma nota fiscal
// através da Chave (chave de idempotência). Itens é guardado como texto
// porque o database/sql não sabe converter []ItemReserva para uma coluna
// sozinho — a conversão para/de JSON é feita manualmente no store.
type Reserva struct {
	Chave  string        `json:"chave"`
	Status string        `json:"status"`
	Itens  []ItemReserva `json:"itens"`
}
