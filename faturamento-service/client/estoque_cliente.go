package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"korp-teste/faturamento-service/models"
)

type EstoqueClient struct {
	baseURL string
	http    *http.Client
}

func NewEstoqueClient(baseURL string) *EstoqueClient {
	return &EstoqueClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}

}

type reservarRequest struct {
	Chave string            `json:"chave"`
	Itens []models.ItemNota `json:"itens"`
}

// reservar chama POST /reservas no estoque-service
func (c *EstoqueClient) Reservar(chave string, itens []models.ItemNota) error {
	body, err := json.Marshal(reservarRequest{Chave: chave, Itens: itens})
	if err != nil {
		return fmt.Errorf("falha ao serializar requisicao de reserva: %w", err)
	}
	resp, err := c.http.Post(c.baseURL+"/reservas", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("falha ao chamar estoque-sevice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("estoque-service recusou a reserva (status %d)", resp.StatusCode)

	}
	return nil
}

// confirmar chama POST /reservas/{chave}/confirmar no estoque-service
func (c *EstoqueClient) Confirmar(chave string) error {
	resp, err := c.http.Post(c.baseURL+"/reservas/"+chave+"/confirmar", "application/json", nil)
	if err != nil {
		return fmt.Errorf("falha ao chamar estoque-service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("estoque-service recusou a confirmacao (status %d)", resp.StatusCode)

	}
	return nil
}

// Cancelar chama POST /reservas/{chave}/cancelar no estoque-service.
func (c *EstoqueClient) Cancelar(chave string) error {
	resp, err := c.http.Post(c.baseURL+"/reservas/"+chave+"/cancelar", "application/json", nil)
	if err != nil {
		return fmt.Errorf("falha ao chamar estoque-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("estoque-service recusou o cancelamento (status %d)", resp.StatusCode)
	}
	return nil
}

// ProdutoResumo tem so os campos que o faturamento-service precisa dos
// produtos do estoque-service (preco, para calcular valor de vendas).
type ProdutoResumo struct {
	Codigo    string  `json:"codigo"`
	Descricao string  `json:"descricao"`
	Preco     float64 `json:"preco"`
}

// ListarProdutos chama GET /produtos no estoque-service.
func (c *EstoqueClient) ListarProdutos() ([]ProdutoResumo, error) {
	resp, err := c.http.Get(c.baseURL + "/produtos")
	if err != nil {
		return nil, fmt.Errorf("falha ao chamar estoque-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("estoque-service recusou a listagem de produtos (status %d)", resp.StatusCode)
	}

	var produtos []ProdutoResumo
	if err := json.NewDecoder(resp.Body).Decode(&produtos); err != nil {
		return nil, fmt.Errorf("falha ao interpretar resposta do estoque-service: %w", err)
	}
	return produtos, nil
}
