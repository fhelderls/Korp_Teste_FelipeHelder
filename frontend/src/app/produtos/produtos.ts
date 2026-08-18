import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CurrencyPipe } from '@angular/common';
import { ProdutosService } from '../services/produtos.service';
import { Produto } from '../models/produto';

@Component({
  selector: 'app-produtos',
  imports: [FormsModule, CurrencyPipe],
  templateUrl: './produtos.html',
  styleUrl: './produtos.css'
})
export class Produtos implements OnInit {
  produtos: ProdutosService['produtos'];
  erro = signal('');

  novoProduto: Produto = { codigo: '', descricao: '', saldo: 0, preco: 0, saldo_reservado: 0, saldo_disponivel: 0 };
  precoTexto = '';

  codigoEditando = signal<string | null>(null);
  edicaoDescricao = '';
  edicaoSaldo = 0;
  edicaoPrecoTexto = '';

  constructor(private produtosService: ProdutosService) {
    this.produtos = this.produtosService.produtos;
  }

  ngOnInit(): void {
    this.produtosService.carregar();
  }

  criar(): void {
    this.erro.set('');

    if (!this.novoProduto.descricao?.trim()) {
      this.erro.set('Preencha a descrição do produto.');
      return;
    }
    if (this.novoProduto.saldo === null || this.novoProduto.saldo === undefined || isNaN(this.novoProduto.saldo) || this.novoProduto.saldo < 0) {
      this.erro.set('Preencha o saldo do produto.');
      return;
    }
    if (!this.precoTexto?.trim()) {
      this.erro.set('Preencha o preço do produto.');
      return;
    }
    const preco = parseFloat(this.precoTexto.replace(',', '.'));
    if (isNaN(preco) || preco <= 0) {
      this.erro.set('Preço precisa ser maior que zero.');
      return;
    }
    this.novoProduto.preco = preco;

    this.produtosService.criar(this.novoProduto).subscribe({
      next: () => {
        this.novoProduto = { codigo: '', descricao: '', saldo: 0, preco: 0, saldo_reservado: 0, saldo_disponivel: 0 };
        this.precoTexto = '';
        this.produtosService.carregar();
      },
      error: (err) => this.erro.set('Falha ao criar produto: ' + err.message)
    });
  }

  deletar(codigo: string): void {
    this.produtosService.deletar(codigo).subscribe({
      next: () => this.produtosService.carregar(),
      error: (err) => this.erro.set('Falha ao deletar produto: ' + (err.error ?? err.message))
    });
  }

  // Edicao inline: usada principalmente pra reajustar o saldo em estoque
  // sem precisar excluir e recriar o produto.
  iniciarEdicao(produto: Produto): void {
    this.erro.set('');
    this.codigoEditando.set(produto.codigo);
    this.edicaoDescricao = produto.descricao;
    this.edicaoSaldo = produto.saldo;
    this.edicaoPrecoTexto = produto.preco.toFixed(2).replace('.', ',');
  }

  cancelarEdicao(): void {
    this.codigoEditando.set(null);
  }

  salvarEdicao(codigo: string): void {
    this.erro.set('');

    if (!this.edicaoDescricao?.trim()) {
      this.erro.set('Preencha a descrição do produto.');
      return;
    }
    if (this.edicaoSaldo === null || this.edicaoSaldo === undefined || isNaN(this.edicaoSaldo) || this.edicaoSaldo < 0) {
      this.erro.set('Preencha o saldo do produto.');
      return;
    }
    if (!this.edicaoPrecoTexto?.trim()) {
      this.erro.set('Preencha o preço do produto.');
      return;
    }
    const preco = parseFloat(this.edicaoPrecoTexto.replace(',', '.'));
    if (isNaN(preco) || preco <= 0) {
      this.erro.set('Preço precisa ser maior que zero.');
      return;
    }

    const produtoAtualizado: Produto = { codigo, descricao: this.edicaoDescricao, saldo: this.edicaoSaldo, preco, saldo_reservado: 0, saldo_disponivel: 0 };
    this.produtosService.atualizar(codigo, produtoAtualizado).subscribe({
      next: () => {
        this.codigoEditando.set(null);
        this.produtosService.carregar();
      },
      error: (err) => this.erro.set('Falha ao atualizar produto: ' + (err.error ?? err.message))
    });
  }
}
