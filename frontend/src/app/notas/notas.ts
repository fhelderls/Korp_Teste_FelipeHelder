import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { NotasService } from '../services/notas.service';
import { ProdutosService } from '../services/produtos.service';
import { NotaFiscal, ItemNota, CriarRequest } from '../models/nota';
import { Produto } from '../models/produto';

@Component({
  selector: 'app-notas',
  imports: [FormsModule],
  templateUrl: './notas.html',
  styleUrl: './notas.css'
})
export class Notas implements OnInit {
  notas = signal<NotaFiscal[]>([]);
  produtos = signal<Produto[]>([]);
  erro = signal('');
  resumoIA = signal('');
  carregandoResumo = signal(false);
  imprimindo = signal<string | null>(null);

  cliente = '';
  itens = signal<ItemNota[]>([{ produto_codigo: '', quantidade: 1 }]);

  constructor(
    private notasService: NotasService,
    private produtosService: ProdutosService
  ) {}

  ngOnInit(): void {
    this.carregar();
    this.produtosService.listar().subscribe({
      next: (produtos) => this.produtos.set(produtos)
    });
  }

  carregar(): void {
    this.notasService.listar().subscribe({
      next: (notas) => this.notas.set(notas),
      error: (err) => this.erro.set('Falha ao carregar notas: ' + err.message)
    });
  }

  descricaoProduto(codigo: string): string {
    return this.produtos().find(p => p.codigo === codigo)?.descricao ?? codigo;
  }

  adicionarItem(): void {
    this.itens.update(itens => [...itens, { produto_codigo: '', quantidade: 1 }]);
  }

  removerItem(index: number): void {
    this.itens.update(itens => itens.filter((_, i) => i !== index));
  }

  atualizarProduto(index: number, valor: string): void {
    this.itens.update(itens =>
      itens.map((item, i) => i === index ? { ...item, produto_codigo: valor } : item)
    );
  }

  atualizarQuantidade(index: number, valor: number): void {
    this.itens.update(itens =>
      itens.map((item, i) => i === index ? { ...item, quantidade: valor } : item)
    );
  }

  criar(): void {
    this.erro.set('');

    if (!this.cliente.trim()) {
      this.erro.set('Preencha o cliente antes de criar a nota.');
      return;
    }
    if (this.itens().some(item => !item.produto_codigo)) {
      this.erro.set('Selecione um produto para cada item antes de criar a nota.');
      return;
    }

    const nota: CriarRequest = {
      cliente: this.cliente,
      itens: this.itens()
    };

    this.notasService.criar(nota).subscribe({
      next: () => {
        this.cliente = '';
        this.itens.set([{ produto_codigo: '', quantidade: 1 }]);
        this.carregar();
      },
      error: (err) => this.erro.set(err.error ?? 'Falha ao criar nota fiscal')
    });
  }

  // Imprimir e a acao que de fato debita o estoque e fecha a nota. So
  // funciona em notas 'Aberta'; se falhar, a nota continua 'Aberta' e o
  // usuario pode clicar em Imprimir de novo.
  imprimir(nota: NotaFiscal): void {
    this.erro.set('');
    this.imprimindo.set(nota.chave);

    this.notasService.imprimir(nota.chave).subscribe({
      next: () => {
        this.imprimindo.set(null);
        this.carregar();
        window.open(this.notasService.pdfUrl(nota.chave), '_blank');
      },
      error: (err) => {
        this.imprimindo.set(null);
        this.erro.set(err.error ?? 'Falha ao imprimir nota fiscal');
        this.carregar();
      }
    });
  }

  pdfUrl(chave: string): string {
    return this.notasService.pdfUrl(chave);
  }

  relatorioUrl(): string {
    return this.notasService.relatorioUrl();
  }

  gerarResumo(): void {
    this.carregandoResumo.set(true);
    this.resumoIA.set('');
    this.notasService.resumo().subscribe({
      next: (resposta) => {
        this.resumoIA.set(resposta.resumo);
        this.carregandoResumo.set(false);
      },
      error: (err) => {
        this.erro.set('Falha ao gerar resumo com IA: ' + (err.error ?? err.message));
        this.carregandoResumo.set(false);
      }
    });
  }
}
