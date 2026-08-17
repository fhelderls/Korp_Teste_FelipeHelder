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
  produtos = signal<Produto[]>([]);
  erro = signal('');

  novoProduto: Produto = { codigo: '', descricao: '', saldo: 0, preco: 0 };
  precoTexto = '';

  constructor(private produtosService: ProdutosService) {}

  ngOnInit(): void {
    this.carregar();
  }

  carregar(): void {
    this.produtosService.listar().subscribe({
      next: (produtos) => this.produtos.set(produtos),
      error: (err) => this.erro.set('Falha ao carregar produtos: ' + err.message)
    });
  }

  criar(): void {
    this.novoProduto.preco = parseFloat(this.precoTexto.replace(',', '.')) || 0;

    this.produtosService.criar(this.novoProduto).subscribe({
      next: () => {
        this.novoProduto = { codigo: '', descricao: '', saldo: 0, preco: 0 };
        this.precoTexto = '';
        this.carregar();
      },
      error: (err) => this.erro.set('Falha ao criar produto: ' + err.message)
    });
  }

  deletar(codigo: string): void {
    this.produtosService.deletar(codigo).subscribe({
      next: () => this.carregar(),
      error: (err) => this.erro.set('Falha ao deletar produto: ' + (err.error ?? err.message))
    });
  }
}
