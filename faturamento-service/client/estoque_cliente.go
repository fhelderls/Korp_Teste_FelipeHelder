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
