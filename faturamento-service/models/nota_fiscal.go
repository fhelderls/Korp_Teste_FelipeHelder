package models

import "time"

// itemNota e um produto e a quantidade dentro de uma nota fiscal.
// Descricao e PrecoUnitario sao um retrato (snapshot) do produto no
// momento em que a nota foi criada - uma nota fiscal e um documento
// historico, e nao pode mudar retroativamente se o produto for editado
// ou excluido depois. Sao preenchidos pelo Criar, nunca pelo cliente.
type ItemNota struct {
	ProdutoCodigo string  `json:"produto_codigo"`
	Quantidade    int     `json:"quantidade"`
	Descricao     string  `json:"descricao"`
	PrecoUnitario float64 `json:"preco_unitario"`
}

// notafiscal representa um pedido de emissao. A Chave e a mesma chave de idempotencia
// usada na chamada de reserva do estoque-service, ligando os dois servicos pelo mesmo identificador
type NotaFiscal struct {
	Chave        string     `json:"chave"`
	Cliente      string     `json:"cliente"`
	Itens        []ItemNota `json:"itens"`
	Status       string     `json:"status"`
	DataAbertura time.Time  `json:"data_abertura"`
	// DataEmissao so e preenchida quando a nota e impressa com sucesso (vira 'Fechada').
	DataEmissao *time.Time `json:"data_emissao"`
}
