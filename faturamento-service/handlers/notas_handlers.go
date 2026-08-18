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

type criarRequest struct {
	Cliente string            `json:"cliente"`
	Itens   []models.ItemNota `json:"itens"`
}

// Criar cadastra uma nota fiscal nova, com numero sequencial gerado
// automaticamente e status 'Aberta'. So grava o cadastro - nao mexe em
// estoque nenhum ainda. A impressao (que debita o estoque de verdade) e
// uma acao separada, feita depois pelo Imprimir.
func (h *NotasHandlers) Criar(w http.ResponseWriter, r *http.Request) {
	var req criarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corpo da requisicao invalido", http.StatusBadRequest)
		return
	}

	if req.Cliente == "" {
		http.Error(w, "cliente e obrigatorio", http.StatusBadRequest)
		return
	}
	if len(req.Itens) == 0 {
		http.Error(w, "a nota precisa de pelo menos um item", http.StatusBadRequest)
		return
	}
	for _, item := range req.Itens {
		if item.ProdutoCodigo == "" {
			http.Error(w, "todo item precisa de um produto selecionado", http.StatusBadRequest)
			return
		}
		if item.Quantidade <= 0 {
			http.Error(w, "a quantidade de cada item precisa ser maior que zero", http.StatusBadRequest)
			return
		}
	}

	nota := models.NotaFiscal{
		Cliente: req.Cliente,
		Itens:   req.Itens,
	}
	if err := h.store.Create(&nota); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(nota)
}

// Imprimir e a acao do botao de impressao: reserva e confirma o estoque
// dos itens da nota e, se der tudo certo, muda o status para 'Fechada'.
// So pode ser chamada em notas 'Aberta' - uma nota 'Fechada' nao pode ser
// impressa de novo. Se a reserva ou a confirmacao falharem, a nota
// simplesmente continua 'Aberta' (nada e alterado), o erro e devolvido
// ao usuario, e ele pode clicar em Imprimir de novo mais tarde - e assim
// que o reprocessamento acontece, sem precisar de uma acao separada.
func (h *NotasHandlers) Imprimir(w http.ResponseWriter, r *http.Request) {
	chave := r.PathValue("chave")

	nota, err := h.store.GetByChave(chave)
	if err == sql.ErrNoRows {
		http.Error(w, "nota fiscal nao encontrada", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if nota.Status != "Aberta" {
		http.Error(w, "nota fiscal nao esta aberta, nao pode ser impressa novamente", http.StatusConflict)
		return
	}

	if err := h.estoqueClient.Reservar(nota.Chave, nota.Itens); err != nil {
		http.Error(w, "nao foi possivel imprimir a nota, falha ao reservar o estoque: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.estoqueClient.Confirmar(nota.Chave); err != nil {
		h.estoqueClient.Cancelar(nota.Chave)
		http.Error(w, "falha ao confirmar a impressao, reserva de estoque cancelada: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.store.AtualizarStatus(nota.Chave, "Fechada"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nota.Status = "Fechada"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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

// dadosAgregados junta os numeros de faturamento (contagem por status,
// quantidade/valor totais e por produto/cliente) usados tanto pelo resumo
// com IA quanto pelo relatorio em PDF - evita calcular a mesma coisa duas
// vezes em handlers diferentes.
type dadosAgregados struct {
	contagem             map[string]int
	quantidadePorProduto map[string]int
	valorPorProduto      map[string]float64
	quantidadePorCliente map[string]int
	valorPorCliente      map[string]float64
	quantidadeTotal      int
	valorTotal           float64
}

func (h *NotasHandlers) agregarDados(notas []models.NotaFiscal) dadosAgregados {
	precos := map[string]float64{}
	descricoes := map[string]string{}
	if produtos, err := h.estoqueClient.ListarProdutos(); err == nil {
		for _, p := range produtos {
			precos[p.Codigo] = p.Preco
			descricoes[p.Codigo] = p.Descricao
		}
	}

	a := dadosAgregados{
		contagem:             map[string]int{},
		quantidadePorProduto: map[string]int{},
		valorPorProduto:      map[string]float64{},
		quantidadePorCliente: map[string]int{},
		valorPorCliente:      map[string]float64{},
	}

	for _, nota := range notas {
		a.contagem[nota.Status]++
		if nota.Status != "Fechada" {
			continue
		}
		for _, item := range nota.Itens {
			// usa a descricao do produto na agregacao (nao o codigo PROD-XXX),
			// pra os relatorios falarem em nomes reais de produto
			nomeProduto := descricoes[item.ProdutoCodigo]
			if nomeProduto == "" {
				nomeProduto = item.ProdutoCodigo
			}

			valorItem := precos[item.ProdutoCodigo] * float64(item.Quantidade)
			a.quantidadeTotal += item.Quantidade
			a.valorTotal += valorItem
			a.quantidadePorProduto[nomeProduto] += item.Quantidade
			a.valorPorProduto[nomeProduto] += valorItem
			a.quantidadePorCliente[nota.Cliente] += item.Quantidade
			a.valorPorCliente[nota.Cliente] += valorItem
		}
	}
	return a
}

// Resumo agrega os dados das notas fechadas (contagem por status, valor
// liquido de vendas, quantidade e valor por produto e por cliente, no
// estilo do painel de insights da Korp) e pede pra IA gerar um resumo
// curto em linguagem natural sobre eles.
func (h *NotasHandlers) Resumo(w http.ResponseWriter, r *http.Request) {
	notas, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a := h.agregarDados(notas)

	prompt := fmt.Sprintf(
		"Voce e um assistente de insights de um sistema de emissao de notas fiscais, no estilo do painel de "+
			"insights de vendas da Korp (ERP). Gere um resumo curto (3 a 5 frases, em portugues, sem introducao, "+
			"direto ao ponto) com base nestes dados: %d notas fechadas (impressas), %d ainda abertas. "+
			"Quantidade total vendida: %d unidades. Valor liquido de vendas total: R$ %.2f. "+
			"Quantidade vendida por produto: %v. Valor de vendas por produto: %v. "+
			"Quantidade vendida por cliente: %v. Valor de vendas por cliente: %v. "+
			"Destaque o produto mais vendido e o cliente com maior valor de compra.",
		a.contagem["Fechada"], a.contagem["Aberta"],
		a.quantidadeTotal, a.valorTotal,
		a.quantidadePorProduto, a.valorPorProduto,
		a.quantidadePorCliente, a.valorPorCliente,
	)

	resumo, err := h.aiClient.Resumir(prompt)
	if err != nil {
		http.Error(w, "falha ao gerar resumo com IA: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"resumo": resumo})
}

// Relatorio gera um PDF de faturamento com os totais gerais e graficos de
// barra horizontal de faturamento por produto e por cliente, no estilo do
// painel de faturamento da Korp (Faturamento por Produto, Faturamento por
// Cliente, Painel de Resumo de Faturamento).
func (h *NotasHandlers) Relatorio(w http.ResponseWriter, r *http.Request) {
	notas, err := h.store.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a := h.agregarDados(notas)

	arquivo, err := pdf.GerarRelatorio(pdf.DadosRelatorio{
		NotasFechadas:   a.contagem["Fechada"],
		NotasAbertas:    a.contagem["Aberta"],
		QuantidadeTotal: a.quantidadeTotal,
		ValorTotal:      a.valorTotal,
		PorProduto:      a.valorPorProduto,
		PorCliente:      a.valorPorCliente,
	})
	if err != nil {
		http.Error(w, "falha ao gerar relatorio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="relatorio-faturamento.pdf"`)
	w.Write(arquivo)
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
