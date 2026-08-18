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
// automaticamente e status 'Aberta', e ja REGISTRA a reserva dos itens
// nesse momento - mas sem verificar saldo, so a intencao de uso fica
// guardada (por isso o saldo_reservado de um produto pode passar do
// saldo total, se varias notas abertas disputarem o mesmo produto). A
// disputa por estoque so e resolvida de fato na emissao. Se o
// estoque-service estiver fora do ar e a reserva nao puder nem ser
// registrada, a criacao da nota inteira e desfeita.
//
// A descricao e o preco de cada item sao um retrato do produto nesse
// momento, gravados junto com a nota: uma nota fiscal e um documento
// historico, e nao pode mudar (ou ficar com o codigo sem descricao)
// se o produto for editado ou excluido depois.
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

	produtosDisponiveis, err := h.estoqueClient.ListarProdutos()
	if err != nil {
		http.Error(w, "nao foi possivel criar a nota, falha ao consultar os produtos: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	produtosPorCodigo := map[string]client.ProdutoResumo{}
	for _, p := range produtosDisponiveis {
		produtosPorCodigo[p.Codigo] = p
	}

	itens := make([]models.ItemNota, len(req.Itens))
	for i, item := range req.Itens {
		produto, existe := produtosPorCodigo[item.ProdutoCodigo]
		if !existe {
			http.Error(w, "produto "+item.ProdutoCodigo+" nao encontrado", http.StatusBadRequest)
			return
		}
		itens[i] = models.ItemNota{
			ProdutoCodigo: item.ProdutoCodigo,
			Quantidade:    item.Quantidade,
			Descricao:     produto.Descricao,
			PrecoUnitario: produto.Preco,
		}
	}

	nota := models.NotaFiscal{
		Cliente: req.Cliente,
		Itens:   itens,
	}
	if err := h.store.Create(&nota); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.estoqueClient.Reservar(nota.Chave, nota.Itens); err != nil {
		h.store.Delete(nota.Chave)
		http.Error(w, "nao foi possivel criar a nota, falha ao registrar a reserva de estoque: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(nota)
}

// Imprimir e a acao do botao de emissao: confirma a reserva de estoque
// que ja foi registrada na criacao da nota. E so agora, na confirmacao,
// que o saldo de verdade e conferido e debitado - se nao houver saldo
// suficiente (porque outra nota concorrente ja consumiu o estoque, por
// exemplo), a emissao falha aqui. So pode ser chamada em notas 'Aberta' -
// uma nota 'Fechada' nao pode ser emitida de novo. Em qualquer falha
// (saldo insuficiente ou estoque-service fora do ar), a nota simplesmente
// continua 'Aberta' com a reserva ainda pendente, o erro e devolvido ao
// usuario, e ele pode clicar em Emitir de novo mais tarde - e assim que o
// reprocessamento acontece, sem precisar de uma acao separada.
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
		http.Error(w, "nota fiscal nao esta aberta, nao pode ser emitida novamente", http.StatusConflict)
		return
	}

	if err := h.estoqueClient.Confirmar(nota.Chave); err != nil {
		http.Error(w, "nao foi possivel emitir a nota, falha ao confirmar a reserva de estoque: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	dataEmissao, err := h.store.MarcarEmitida(nota.Chave)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nota.Status = "Fechada"
	nota.DataEmissao = &dataEmissao

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nota)
}

// Cancelar desiste de uma nota fiscal 'Aberta', liberando a reserva de
// estoque correspondente e removendo a nota. So funciona em notas
// 'Aberta' - uma nota 'Fechada' e definitiva, ja foi emitida de verdade
// e nao pode ser desfeita por aqui.
func (h *NotasHandlers) Cancelar(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "nota fiscal nao esta aberta, nao pode ser cancelada", http.StatusConflict)
		return
	}

	if err := h.estoqueClient.Cancelar(nota.Chave); err != nil {
		http.Error(w, "nao foi possivel cancelar a nota, falha ao liberar a reserva de estoque: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	if err := h.store.Delete(nota.Chave); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
			// usa a descricao gravada na nota na agregacao (nao o codigo
			// PROD-XXX), pra os relatorios falarem em nomes reais de produto
			nomeProduto := item.Descricao
			if nomeProduto == "" {
				nomeProduto = item.ProdutoCodigo
			}

			valorItem := item.PrecoUnitario * float64(item.Quantidade)
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
		NotasFechadas:        a.contagem["Fechada"],
		NotasAbertas:         a.contagem["Aberta"],
		QuantidadeTotal:      a.quantidadeTotal,
		ValorTotal:           a.valorTotal,
		PorProduto:           a.valorPorProduto,
		PorCliente:           a.valorPorCliente,
		QuantidadePorProduto: a.quantidadePorProduto,
		QuantidadePorCliente: a.quantidadePorCliente,
	})
	if err != nil {
		http.Error(w, "falha ao gerar relatorio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="relatorio-faturamento.pdf"`)
	w.Write(arquivo)
}

// PDF gera e devolve o PDF de uma nota fiscal, usando a descricao e o
// preco gravados na propria nota (o retrato do produto no momento da
// criacao, nao o estado atual do estoque-service).
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

	arquivo, err := pdf.GerarNotaFiscal(nota)
	if err != nil {
		http.Error(w, "falha ao gerar PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="nota-%s.pdf"`, nota.Chave))
	w.Write(arquivo)
}
