package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"korp-teste/faturamento-service/models"
)

type EstoqueClient struct {
	baseURL string
	http    *http.Client
}

func NewEstoqueClient(baseURL string) *EstoqueClient {
	return &EstoqueClient{
		baseURL: baseURL,
		// timeout curto por tentativa: sem isso, o http.Client do Go espera
		// indefinidamente por uma conexao que nunca vai completar quando o
		// estoque-service esta fora do ar, o que tornaria o retry muito lento
		http: &http.Client{Timeout: 2 * time.Second},
	}

}

// comRetry tenta rodar fn ate tentativas vezes, com backoff exponencial
// entre cada tentativa (200ms, 400ms, 800ms...). So faz sentido pra falhas
// de rede (servico fora do ar, timeout) - nao deve ser usado para erros de
// regra de negocio (ex: saldo insuficiente), que nao se resolvem tentando
// de novo.
func comRetry(tentativas int, fn func() error) error {
	var err error
	espera := 200 * time.Millisecond
	for i := 0; i < tentativas; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i < tentativas-1 {
			time.Sleep(espera)
			espera *= 2
		}
	}
	return err
}

type reservarRequest struct {
	Chave string            `json:"chave"`
	Itens []models.ItemNota `json:"itens"`
}

// Reservar chama POST /reservas no estoque-service. A chamada de rede em
// si tem retry com backoff (o servico pode estar reiniciando bem nesse
// instante); uma recusa de negocio (status != 201) nao e reprocessada.
func (c *EstoqueClient) Reservar(chave string, itens []models.ItemNota) error {
	body, err := json.Marshal(reservarRequest{Chave: chave, Itens: itens})
	if err != nil {
		return fmt.Errorf("falha ao serializar requisicao de reserva: %w", err)
	}

	var resp *http.Response
	err = comRetry(3, func() error {
		var errReq error
		resp, errReq = c.http.Post(c.baseURL+"/reservas", "application/json", bytes.NewReader(body))
		return errReq
	})
	if err != nil {
		return fmt.Errorf("falha ao chamar estoque-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("estoque-service recusou a reserva (status %d)", resp.StatusCode)
	}
	return nil
}

// Confirmar chama POST /reservas/{chave}/confirmar no estoque-service, com
// o mesmo retry de rede do Reservar.
func (c *EstoqueClient) Confirmar(chave string) error {
	var resp *http.Response
	err := comRetry(3, func() error {
		var errReq error
		resp, errReq = c.http.Post(c.baseURL+"/reservas/"+chave+"/confirmar", "application/json", nil)
		return errReq
	})
	if err != nil {
		return fmt.Errorf("falha ao chamar estoque-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("estoque-service recusou a confirmacao (status %d)", resp.StatusCode)
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
