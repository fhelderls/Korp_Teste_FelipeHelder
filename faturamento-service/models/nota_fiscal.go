package models

//itemNota e um produto e a quantidade dentro de uma nota fiscal

type ItemNota struct {
	ProdutoCodigo string `json:"produto_codigo"`
	Quantidade    int    `json:"quantidade"`
}

//notafiscal representa um pedido de emissao. A Chave e a mesma chave de idempotencia
//usada na chamada de reserva do estoque-service, ligando os dois servicos pelo mesmo identificador
type NotaFiscal struct {
	Chave   string     `json:"chave"`
	Cliente string     `json:"cliente"`
	Itens   []ItemNota `json:"itens"`
	Status  string     `json:"status"`
}
