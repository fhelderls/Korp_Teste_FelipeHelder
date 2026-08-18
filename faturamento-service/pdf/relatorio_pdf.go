package pdf

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// DadosRelatorio junta os numeros de faturamento usados pra montar o
// relatorio em PDF.
type DadosRelatorio struct {
	NotasFechadas        int
	NotasAbertas         int
	QuantidadeTotal      int
	ValorTotal           float64
	PorProduto           map[string]float64
	PorCliente           map[string]float64
	QuantidadePorProduto map[string]int
	QuantidadePorCliente map[string]int
}

type itemOrdenado struct {
	Rotulo string
	Valor  float64
}

// corPrimaria e a cor de destaque usada no cabecalho e nas barras/fatias
// principais do relatorio.
var corPrimaria = [3]int{37, 65, 135}

// paletaCores da as cores das fatias de pizza e da legenda, na ordem de
// uso (a ultima entrada e reservada pro item "Outros").
var paletaCores = [][3]int{
	{61, 111, 242},
	{242, 153, 61},
	{61, 189, 141},
	{217, 83, 121},
	{155, 107, 224},
	{242, 201, 61},
	{61, 189, 242},
	{160, 160, 160},
}

// ordenarTopN transforma um mapa rotulo->valor numa lista ordenada do
// maior valor pro menor, limitada aos n primeiros.
func ordenarTopN(dados map[string]float64, n int) []itemOrdenado {
	itens := paraLista(dados)
	if len(itens) > n {
		itens = itens[:n]
	}
	return itens
}

// agruparTopNComOutros e igual ao ordenarTopN, mas soma o restante dos
// itens (alem dos n maiores) num item "Outros", pra pizza sempre
// representar 100% do total, mesmo com muitos produtos/clientes.
func agruparTopNComOutros(dados map[string]float64, n int) []itemOrdenado {
	itens := paraLista(dados)
	if len(itens) <= n {
		return itens
	}
	resultado := append([]itemOrdenado{}, itens[:n]...)
	var outros float64
	for _, item := range itens[n:] {
		outros += item.Valor
	}
	if outros > 0 {
		resultado = append(resultado, itemOrdenado{Rotulo: "Outros", Valor: outros})
	}
	return resultado
}

func paraLista(dados map[string]float64) []itemOrdenado {
	itens := make([]itemOrdenado, 0, len(dados))
	for rotulo, valor := range dados {
		itens = append(itens, itemOrdenado{Rotulo: rotulo, Valor: valor})
	}
	sort.Slice(itens, func(i, j int) bool { return itens[i].Valor > itens[j].Valor })
	return itens
}

// paraListaInt converte um mapa de quantidades (int) pro mesmo formato
// usado nos graficos, jah ordenado do maior pro menor e limitado a n.
func paraListaInt(dados map[string]int, n int) []itemOrdenado {
	convertido := make(map[string]float64, len(dados))
	for rotulo, valor := range dados {
		convertido[rotulo] = float64(valor)
	}
	return ordenarTopN(convertido, n)
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

// GerarRelatorio monta o dashboard de faturamento em PDF: cabecalho,
// indicadores-chave, destaques (produto/cliente com maior faturamento),
// graficos de pizza com a participacao no faturamento e graficos de
// barra com a quantidade vendida - no estilo do painel de insights de
// vendas da Korp.
func GerarRelatorio(dados DadosRelatorio) ([]byte, error) {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetAutoPageBreak(false, 0)
	doc.AddPage()

	cabecalho(doc)
	cartoesResumo(doc, dados)
	destaques(doc, dados)

	produtosPorValor := agruparTopNComOutros(dados.PorProduto, 6)
	clientesPorValor := agruparTopNComOutros(dados.PorCliente, 6)
	desenharPizza(doc, "Participacao no Faturamento por Produto", produtosPorValor, dados.ValorTotal)

	doc.AddPage()
	cabecalhoSecundario(doc, "Faturamento por Cliente e Quantidades Vendidas")
	desenharPizza(doc, "Participacao no Faturamento por Cliente", clientesPorValor, dados.ValorTotal)

	desenharBarras(doc, "Quantidade Vendida por Produto", paraListaInt(dados.QuantidadePorProduto, 8), "un")
	desenharBarras(doc, "Quantidade Vendida por Cliente", paraListaInt(dados.QuantidadePorCliente, 8), "un")

	rodape(doc)

	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, fmt.Errorf("falha ao gerar PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// cabecalho desenha a faixa colorida do topo da primeira pagina, com
// titulo, subtitulo e a data/hora de geracao do relatorio.
func cabecalho(doc *fpdf.Fpdf) {
	doc.SetFillColor(corPrimaria[0], corPrimaria[1], corPrimaria[2])
	doc.Rect(0, 0, 210, 32, "F")

	doc.SetTextColor(255, 255, 255)
	doc.SetFont("Arial", "B", 20)
	doc.SetXY(12, 8)
	doc.Cell(0, 10, "Relatorio de Faturamento")

	doc.SetFont("Arial", "", 11)
	doc.SetXY(12, 19)
	doc.Cell(0, 6, "Sistema de Emissao de Notas Fiscais")

	doc.SetFont("Arial", "", 9)
	doc.SetXY(12, 25)
	doc.Cell(0, 5, "Gerado em "+time.Now().Format("02/01/2006 as 15:04"))

	doc.SetTextColor(0, 0, 0)
	doc.SetY(40)
}

// cabecalhoSecundario desenha um cabecalho mais simples, usado nas
// paginas seguintes a primeira.
func cabecalhoSecundario(doc *fpdf.Fpdf, titulo string) {
	doc.SetFillColor(corPrimaria[0], corPrimaria[1], corPrimaria[2])
	doc.Rect(0, 0, 210, 18, "F")

	doc.SetTextColor(255, 255, 255)
	doc.SetFont("Arial", "B", 13)
	doc.SetXY(12, 6)
	doc.Cell(0, 8, titulo)

	doc.SetTextColor(0, 0, 0)
	doc.SetY(26)
}

// cartoesResumo desenha uma linha de 4 cartoes com os indicadores-chave
// do periodo: notas fechadas, notas abertas, ticket medio e clientes
// atendidos.
func cartoesResumo(doc *fpdf.Fpdf, dados DadosRelatorio) {
	ticketMedio := 0.0
	if dados.NotasFechadas > 0 {
		ticketMedio = dados.ValorTotal / float64(dados.NotasFechadas)
	}
	clientesAtendidos := len(dados.PorCliente)

	cartoes := []struct {
		rotulo string
		valor  string
	}{
		{"Notas Fechadas", fmt.Sprintf("%d", dados.NotasFechadas)},
		{"Notas Abertas", fmt.Sprintf("%d", dados.NotasAbertas)},
		{"Ticket Medio", fmt.Sprintf("R$ %.2f", ticketMedio)},
		{"Clientes Atendidos", fmt.Sprintf("%d", clientesAtendidos)},
	}

	const margem = 12.0
	const espaco = 4.0
	largura := (210.0 - 2*margem - 3*espaco) / 4
	y := doc.GetY()

	for i, cartao := range cartoes {
		x := margem + float64(i)*(largura+espaco)
		doc.SetDrawColor(220, 220, 220)
		doc.SetFillColor(247, 248, 250)
		doc.Rect(x, y, largura, 20, "FD")

		doc.SetTextColor(120, 120, 120)
		doc.SetFont("Arial", "", 8)
		doc.SetXY(x+3, y+3)
		doc.CellFormat(largura-6, 5, cartao.rotulo, "", 0, "", false, 0, "")

		doc.SetTextColor(corPrimaria[0], corPrimaria[1], corPrimaria[2])
		doc.SetFont("Arial", "B", 13)
		doc.SetXY(x+3, y+10)
		doc.CellFormat(largura-6, 7, cartao.valor, "", 0, "", false, 0, "")
	}

	doc.SetTextColor(0, 0, 0)
	doc.SetY(y + 26)

	doc.SetFont("Arial", "B", 15)
	doc.SetX(margem)
	doc.Cell(0, 9, fmt.Sprintf("Valor liquido de vendas: R$ %.2f", dados.ValorTotal))
	doc.Ln(6)
	doc.SetFont("Arial", "", 10)
	doc.SetX(margem)
	doc.Cell(0, 6, fmt.Sprintf("Quantidade total vendida: %d unidades", dados.QuantidadeTotal))
	doc.Ln(10)
}

// destaques mostra o produto e o cliente com maior faturamento, com o
// percentual de participacao no valor total vendido.
func destaques(doc *fpdf.Fpdf, dados DadosRelatorio) {
	produtos := ordenarTopN(dados.PorProduto, 1)
	clientes := ordenarTopN(dados.PorCliente, 1)
	if len(produtos) == 0 && len(clientes) == 0 {
		return
	}

	const margem = 12.0
	doc.SetFont("Arial", "B", 10)
	doc.SetX(margem)
	doc.SetTextColor(80, 80, 80)
	doc.Cell(0, 6, "Destaques")
	doc.Ln(6)

	doc.SetFont("Arial", "", 10)
	doc.SetTextColor(0, 0, 0)
	if len(produtos) > 0 {
		percentual := percentualDe(produtos[0].Valor, dados.ValorTotal)
		doc.SetX(margem)
		doc.Cell(0, 6, fmt.Sprintf("- Produto mais vendido: %s (R$ %.2f, %.1f%% do faturamento)", produtos[0].Rotulo, produtos[0].Valor, percentual))
		doc.Ln(6)
	}
	if len(clientes) > 0 {
		percentual := percentualDe(clientes[0].Valor, dados.ValorTotal)
		doc.SetX(margem)
		doc.Cell(0, 6, fmt.Sprintf("- Cliente com maior faturamento: %s (R$ %.2f, %.1f%% do faturamento)", clientes[0].Rotulo, clientes[0].Valor, percentual))
		doc.Ln(10)
	}
}

func percentualDe(valor, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (valor / total) * 100
}

// desenharPizza desenha um grafico de pizza com o titulo passado e uma
// legenda ao lado com o rotulo, o percentual e o valor de cada fatia.
func desenharPizza(doc *fpdf.Fpdf, titulo string, dados []itemOrdenado, total float64) {
	const margem = 12.0
	doc.SetFont("Arial", "B", 12)
	doc.SetTextColor(0, 0, 0)
	doc.SetX(margem)
	doc.Cell(0, 8, titulo)
	doc.Ln(10)

	if len(dados) == 0 || total <= 0 {
		doc.SetFont("Arial", "", 10)
		doc.SetX(margem)
		doc.Cell(0, 6, "Sem dados.")
		doc.Ln(10)
		return
	}

	const raio = 26.0
	topo := doc.GetY()
	cx := margem + raio + 4
	cy := topo + raio

	anguloAtual := -90.0
	for i, item := range dados {
		cor := paletaCores[i%len(paletaCores)]
		fracao := item.Valor / total
		varredura := fracao * 360
		desenharFatia(doc, cx, cy, raio, anguloAtual, anguloAtual+varredura, cor)
		anguloAtual += varredura
	}

	legendaX := cx + raio + 14
	legendaY := topo + 1
	doc.SetFont("Arial", "", 9)
	for i, item := range dados {
		cor := paletaCores[i%len(paletaCores)]
		doc.SetFillColor(cor[0], cor[1], cor[2])
		doc.Rect(legendaX, legendaY, 3.5, 3.5, "F")

		percentual := percentualDe(item.Valor, total)
		doc.SetXY(legendaX+6, legendaY-1.3)
		doc.CellFormat(68, 6, truncarRunas(item.Rotulo, 24)+fmt.Sprintf(" (%.1f%%)", percentual), "", 0, "", false, 0, "")
		doc.SetXY(legendaX+74, legendaY-1.3)
		doc.CellFormat(32, 6, fmt.Sprintf("R$ %.2f", item.Valor), "", 1, "", false, 0, "")
		legendaY += 6.5
	}

	fimGrafico := topo + 2*raio + 6
	fimLegenda := legendaY + 4
	fim := math.Max(fimGrafico, fimLegenda)
	doc.SetXY(margem, fim)
}

// desenharFatia desenha uma unica fatia da pizza (poligono do centro ate
// os pontos do arco entre anguloInicio e anguloFim, em graus).
func desenharFatia(doc *fpdf.Fpdf, cx, cy, raio, anguloInicio, anguloFim float64, cor [3]int) {
	doc.SetFillColor(cor[0], cor[1], cor[2])

	const passo = 3.0
	pontos := []fpdf.PointType{{X: cx, Y: cy}}
	for a := anguloInicio; a < anguloFim; a += passo {
		pontos = append(pontos, pontoCirculo(cx, cy, raio, a))
	}
	pontos = append(pontos, pontoCirculo(cx, cy, raio, anguloFim))

	doc.Polygon(pontos, "F")
}

func pontoCirculo(cx, cy, raio, anguloGraus float64) fpdf.PointType {
	rad := anguloGraus * math.Pi / 180
	return fpdf.PointType{X: cx + raio*math.Cos(rad), Y: cy + raio*math.Sin(rad)}
}

// desenharBarras desenha um grafico de barras horizontal simples: uma
// linha por item, com o rotulo a esquerda, uma barra proporcional ao
// maior valor da lista, e o valor (com o sufixo indicado) a direita.
func desenharBarras(doc *fpdf.Fpdf, titulo string, dados []itemOrdenado, sufixo string) {
	const margem = 12.0
	doc.SetFont("Arial", "B", 12)
	doc.SetTextColor(0, 0, 0)
	doc.SetX(margem)
	doc.Cell(0, 8, titulo)
	doc.Ln(9)

	if len(dados) == 0 {
		doc.SetFont("Arial", "", 10)
		doc.SetX(margem)
		doc.Cell(0, 6, "Sem dados.")
		doc.Ln(10)
		return
	}

	maior := dados[0].Valor
	const larguraMaxima = 90.0

	doc.SetFont("Arial", "", 9)
	for _, item := range dados {
		yInicio := doc.GetY()

		doc.SetX(margem)
		doc.CellFormat(55, 6, truncarRunas(item.Rotulo, 28), "", 0, "", false, 0, "")

		largura := 0.0
		if maior > 0 {
			largura = (item.Valor / maior) * larguraMaxima
		}
		x := doc.GetX()
		doc.SetFillColor(corPrimaria[0], corPrimaria[1], corPrimaria[2])
		doc.Rect(x, yInicio+1, largura, 4, "F")

		doc.SetXY(x+larguraMaxima+3, yInicio)
		doc.CellFormat(35, 6, fmt.Sprintf("%.0f %s", item.Valor, sufixo), "", 1, "", false, 0, "")
	}
	doc.Ln(8)
}

// rodape escreve uma linha de rodape na ultima pagina gerada.
func rodape(doc *fpdf.Fpdf) {
	doc.SetY(-18)
	doc.SetFont("Arial", "I", 8)
	doc.SetTextColor(150, 150, 150)
	doc.CellFormat(0, 6, "Korp - Sistema de Emissao de Notas Fiscais", "", 0, "C", false, 0, "")
	doc.SetTextColor(0, 0, 0)
}
