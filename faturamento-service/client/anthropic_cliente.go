package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type AnthropicClient struct {
	apiKey string
	http   *http.Client
}

func NewAnthropicClient(apiKey string) *AnthropicClient {
	return &AnthropicClient{apiKey: apiKey, http: &http.Client{}}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// Resumir manda o prompt para a API da Anthropic (modelo Claude Haiku) e
// devolve o texto gerado.
func (c *AnthropicClient) Resumir(prompt string) (string, error) {
	body, err := json.Marshal(anthropicRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 300,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("falha ao serializar requisicao para a IA: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("falha ao montar requisicao para a IA: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("falha ao chamar a IA: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var corpo bytes.Buffer
		corpo.ReadFrom(resp.Body)
		return "", fmt.Errorf("IA recusou a requisicao (status %d): %s", resp.StatusCode, corpo.String())
	}

	var parsed anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("falha ao interpretar resposta da IA: %w", err)
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("resposta da IA veio vazia")
	}
	return parsed.Content[0].Text, nil
}
