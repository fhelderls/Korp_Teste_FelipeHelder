package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"korp-teste/faturamento-service/client"
	"korp-teste/faturamento-service/models"
	"korp-teste/faturamento-service/pdf"
	"korp-teste/faturamento-service/store"
)

type NotasHandlers struct {
	store         *store.NotasStore
	estoqueClient *client.EstoqueClient
	aiClient      *client.AnthropicClient
}

func NewNotasHandlers(s *store.NotasStore, ec *client.EstoqueClient, ai *client.AnthropicClient) *NotasHandlers {
	return &NotasHandlers{store: s, estoqueClient: ec, aiClient: ai}
}

type emitirRequest struct {
	Chave   string            `json:"chave"`
	Cliente string            `json:"cliente"`
	Itens   []models.ItemNota `json:"itens"`
}

// Emitir orquestra a emissao de uma nota fiscal: grava a nota, reserva o
// estoque, confirma a reserva. Se qualquer etapa falhar depois da reserva,
// cancela a reserva para devolver o estoque (compensacao). Se a chave ja
// existir, reaproveita a nota existente em vez de tentar criar de novo,
// permitindo que o cliente reenvie a mesma requisicao com seguranca.
func (h *NotasHandlers) Emitir(w http.ResponseWriter, r *http.Request) {
	var req emitirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo da requisicao invalido", http.StatusBadRequest)
		return
	}

	nota, err := h.store.GetByChave(req.Chave)
	if err == sql.ErrNoRows {
		nota = models.NotaFiscal{
			Chave:   req.Chave,
			Cliente: req.Cliente,
			Itens:   req.Itens,
		}
		if err := h.store.Create(&nota); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if nota.Status == "emitida" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(nota)
		return
	}

	if err := h.estoqueClient.Reservar(nota.Chave, nota.Itens); err != nil {
		h.store.AtualizarStatus(nota.Chave, "falha")
		http.Error(w, "nao foi possivel reservar o estoque, nota marcada como falha: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.estoqueClient.Confirmar(nota.Chave); err != nil {
		h.estoqueClient.Cancelar(nota.Chave)
		h.store.AtualizarStatus(nota.Chave, "falha")
		http.Error(w, "falha ao confirmar a emissao, reserva de estoque cancelada: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	h.store.AtualizarStatus(nota.Chave, "emitida")
	nota.Status = "emitida"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(nota)
}

// List retorna todas as notas fiscais cadastradas.
func (h *NotasHandlers) List(w http.ResponseWriter, r *http.Request) {
	notas, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notas)
}

// Resumo agrega os dados das notas emitidas (contagem por status, valor
// liquido de vendas, quantidade e valor por produto e por cliente, no
// estilo do painel de insights da Korp) e pede pra IA gerar um resumo
// curto em linguagem natural sobre eles.
func (h *NotasHandlers) Resumo(w http.ResponseWriter, r *http.Request) {
	notas, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	precos := map[string]float64{}
	if produtos, err := h.estoqueClient.ListarProdutos(); err == nil {
		for _, p := range produtos {
			precos[p.Codigo] = p.Preco
		}
	}

	contagem := map[string]int{}
	quantidadePorProduto := map[string]int{}
	valorPorProduto := map[string]float64{}
	quantidadePorCliente := map[string]int{}
	valorPorCliente := map[string]float64{}
	quantidadeTotal := 0
	valorTotal := 0.0

	for _, nota := range notas {
		contagem[nota.Status]++
		if nota.Status != "emitida" {
			continue
		}
		for _, item := range nota.Itens {
			valorItem := precos[item.ProdutoCodigo] * float64(item.Quantidade)
			quantidadeTotal += item.Quantidade
			valorTotal += valorItem
			quantidadePorProduto[item.ProdutoCodigo] += item.Quantidade
			valorPorProduto[item.ProdutoCodigo] += valorItem
			quantidadePorCliente[nota.Cliente] += item.Quantidade
			valorPorCliente[nota.Cliente] += valorItem
		}
	}

	prompt := fmt.Sprintf(
		"Voce e um assistente de insights de um sistema de emissao de notas fiscais, no estilo do painel de "+
			"insights de vendas da Korp (ERP). Gere um resumo curto (3 a 5 frases, em portugues, sem introducao, "+
			"direto ao ponto) com base nestes dados: %d notas emitidas, %d com falha, %d pendentes. "+
			"Quantidade total vendida: %d unidades. Valor liquido de vendas total: R$ %.2f. "+
			"Quantidade vendida por produto: %v. Valor de vendas por produto: %v. "+
			"Quantidade vendida por cliente: %v. Valor de vendas por cliente: %v. "+
			"Destaque o produto mais vendido e o cliente com maior valor de compra.",
		contagem["emitida"], contagem["falha"], contagem["pendente"],
		quantidadeTotal, valorTotal,
		quantidadePorProduto, valorPorProduto,
		quantidadePorCliente, valorPorCliente,
	)

	resumo, err := h.aiClient.Resumir(prompt)
	if err != nil {
		http.Error(w, "falha ao gerar resumo com IA: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"resumo": resumo})
}

// PDF gera e devolve o PDF de uma nota fiscal, buscando a descricao e o
// preco atual dos produtos no estoque-service para montar a tabela.
func (h *NotasHandlers) PDF(w http.ResponseWriter, r *http.Request) {
	chave := r.PathValue("chave")

	nota, err := h.store.GetByChave(chave)
	if err == sql.ErrNoRows {
		http.Error(w, "nota fiscal nao encontrada", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	produtosInfo := map[string]pdf.InfoProduto{}
	if produtos, err := h.estoqueClient.ListarProdutos(); err == nil {
		for _, p := range produtos {
			produtosInfo[p.Codigo] = pdf.InfoProduto{Descricao: p.Descricao, Preco: p.Preco}
		}
	}

	arquivo, err := pdf.GerarNotaFiscal(nota, produtosInfo)
	if err != nil {
		http.Error(w, "falha ao gerar PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="nota-%s.pdf"`, nota.Chave))
	w.Write(arquivo)
}
