package pdf

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/go-pdf/fpdf"
)

// DadosRelatorio junta os numeros de faturamento usados pra montar o
// relatorio em PDF.
type DadosRelatorio struct {
	NotasFechadas   int
	NotasAbertas    int
	QuantidadeTotal int
	ValorTotal      float64
	PorProduto      map[string]float64
	PorCliente      map[string]float64
}

type itemOrdenado struct {
	Rotulo string
	Valor  float64
}

// ordenarTopN transforma um mapa rotulo->valor numa lista ordenada do
// maior valor pro menor, limitada aos n primeiros.
func ordenarTopN(dados map[string]float64, n int) []itemOrdenado {
	itens := make([]itemOrdenado, 0, len(dados))
	for rotulo, valor := range dados {
		itens = append(itens, itemOrdenado{Rotulo: rotulo, Valor: valor})
	}
	sort.Slice(itens, func(i, j int) bool { return itens[i].Valor > itens[j].Valor })
	if len(itens) > n {
		itens = itens[:n]
	}
	return itens
}

// truncarRunas corta a string em ate max caracteres (contando runas, nao
// bytes, pra nao quebrar acentos no meio) e adiciona "..." se cortou algo.
func truncarRunas(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}

// GerarRelatorio monta um PDF de faturamento: totais gerais no topo, e
// dois graficos de barra horizontal (faturamento por produto e por
// cliente), no estilo do painel de insights de vendas da Korp.
func GerarRelatorio(dados DadosRelatorio) ([]byte, error) {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.AddPage()

	doc.SetFont("Arial", "B", 18)
	doc.Cell(0, 12, "Relatorio de Faturamento")
	doc.Ln(14)

	doc.SetFont("Arial", "", 11)
	doc.Cell(0, 7, fmt.Sprintf("Notas fechadas: %d   |   Notas abertas: %d", dados.NotasFechadas, dados.NotasAbertas))
	doc.Ln(6)
	doc.Cell(0, 7, fmt.Sprintf("Quantidade total vendida: %d unidades", dados.QuantidadeTotal))
	doc.Ln(8)
	doc.SetFont("Arial", "B", 13)
	doc.Cell(0, 8, fmt.Sprintf("Valor liquido de vendas: R$ %.2f", dados.ValorTotal))
	doc.Ln(14)

	desenharBarras(doc, "Faturamento por Produto", ordenarTopN(dados.PorProduto, 8))
	desenharBarras(doc, "Faturamento por Cliente", ordenarTopN(dados.PorCliente, 8))

	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, fmt.Errorf("falha ao gerar PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// desenharBarras desenha um grafico de barras horizontal simples: uma
// linha por item, com o rotulo a esquerda, uma barra proporcional ao maior
// valor da lista, e o valor formatado a direita.
func desenharBarras(doc *fpdf.Fpdf, titulo string, dados []itemOrdenado) {
	doc.SetFont("Arial", "B", 12)
	doc.Cell(0, 8, titulo)
	doc.Ln(9)

	if len(dados) == 0 {
		doc.SetFont("Arial", "", 10)
		doc.Cell(0, 6, "Sem dados.")
		doc.Ln(10)
		return
	}

	maior := dados[0].Valor
	const larguraMaxima = 90.0

	doc.SetFont("Arial", "", 9)
	for _, item := range dados {
		yInicio := doc.GetY()

		doc.CellFormat(55, 6, truncarRunas(item.Rotulo, 28), "", 0, "", false, 0, "")

		largura := 0.0
		if maior > 0 {
			largura = (item.Valor / maior) * larguraMaxima
		}
		x := doc.GetX()
		doc.SetFillColor(61, 111, 242)
		doc.Rect(x, yInicio+1, largura, 4, "F")

		doc.SetXY(x+larguraMaxima+3, yInicio)
		doc.CellFormat(35, 6, fmt.Sprintf("R$ %.2f", item.Valor), "", 1, "", false, 0, "")
	}
	doc.Ln(8)
}
