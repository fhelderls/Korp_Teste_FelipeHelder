package models

//produto representa um item de estoque e a quantidade disponível
type Produto struct {
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao"`
	Saldo     int    `json:"saldo"`
}
