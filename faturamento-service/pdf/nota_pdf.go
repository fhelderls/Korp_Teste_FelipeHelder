package pdf

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"

	"korp-teste/faturamento-service/models"
)

const layoutData = "02/01/2006 15:04"

// InfoProduto tem os dados do produto necessarios para exibir no PDF
// (o faturamento-service nao guarda descricao/preco, so o codigo).
type InfoProduto struct {
	Descricao string
	Preco     float64
}

// GerarNotaFiscal monta um PDF simples da nota fiscal: cabecalho com
// chave/cliente/status, tabela de itens com preco unitario e subtotal,
// e o total geral.
func GerarNotaFiscal(nota models.NotaFiscal, produtos map[string]InfoProduto) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(0, 12, "Nota Fiscal")
	pdf.Ln(14)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 7, fmt.Sprintf("Chave: %s", nota.Chave))
	pdf.Ln(6)
	pdf.Cell(0, 7, fmt.Sprintf("Cliente: %s", nota.Cliente))
	pdf.Ln(6)
	pdf.Cell(0, 7, fmt.Sprintf("Status: %s", nota.Status))
	pdf.Ln(6)
	pdf.Cell(0, 7, fmt.Sprintf("Data de abertura: %s", nota.DataAbertura.Format(layoutData)))
	pdf.Ln(6)
	if nota.DataEmissao != nil {
		pdf.Cell(0, 7, fmt.Sprintf("Data de emissao: %s", nota.DataEmissao.Format(layoutData)))
	} else {
		pdf.Cell(0, 7, "Data de emissao: -")
	}
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(80, 8, "Produto", "1", 0, "", true, 0, "")
	pdf.CellFormat(30, 8, "Quantidade", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 8, "Preco unit.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, 8, "Subtotal", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	var total float64
	for _, item := range nota.Itens {
		info := produtos[item.ProdutoCodigo]
		descricao := info.Descricao
		if descricao == "" {
			descricao = item.ProdutoCodigo
		}
		subtotal := info.Preco * float64(item.Quantidade)
		total += subtotal

		pdf.CellFormat(80, 8, descricao, "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%d", item.Quantidade), "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, 8, fmt.Sprintf("R$ %.2f", info.Preco), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 8, fmt.Sprintf("R$ %.2f", subtotal), "1", 1, "R", false, 0, "")
	}

	pdf.SetFont("Arial", "B", 11)
	pdf.Ln(4)
	pdf.CellFormat(145, 8, "Total", "", 0, "R", false, 0, "")
	pdf.CellFormat(35, 8, fmt.Sprintf("R$ %.2f", total), "", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("falha ao gerar PDF: %w", err)
	}
	return buf.Bytes(), nil
}
