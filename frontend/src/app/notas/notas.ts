import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { NotasService } from '../services/notas.service';
import { NotaFiscal, ItemNota } from '../models/nota';

@Component({
  selector: 'app-notas',
  imports: [FormsModule],
  templateUrl: './notas.html',
  styleUrl: './notas.css'
})
export class Notas implements OnInit {
  notas = signal<NotaFiscal[]>([]);
  erro = signal('');
  resumoIA = signal('');
  carregandoResumo = signal(false);

  chave = '';
  cliente = '';
  itens = signal<ItemNota[]>([{ produto_codigo: '', quantidade: 1 }]);

  constructor(private notasService: NotasService) {}

  ngOnInit(): void {
    this.carregar();
  }

  carregar(): void {
    this.notasService.listar().subscribe({
      next: (notas) => this.notas.set(notas),
      error: (err) => this.erro.set('Falha ao carregar notas: ' + err.message)
    });
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

  emitir(): void {
    this.erro.set('');
    const nota: NotaFiscal = {
      chave: this.chave,
      cliente: this.cliente,
      itens: this.itens(),
      status: ''
    };

    this.notasService.emitir(nota).subscribe({
      next: () => {
        this.chave = '';
        this.cliente = '';
        this.itens.set([{ produto_codigo: '', quantidade: 1 }]);
        this.carregar();
      },
      error: (err) => {
        this.erro.set(err.error ?? 'Falha ao emitir nota fiscal');
        this.carregar();
      }
    });
  }

  pdfUrl(chave: string): string {
    return `http://localhost:8082/notas/${chave}/pdf`;
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
