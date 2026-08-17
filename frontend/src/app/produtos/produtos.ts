import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ProdutosService } from '../services/produtos.service';
import { Produto } from '../models/produto';

@Component({
  selector: 'app-produtos',
  imports: [FormsModule],
  templateUrl: './produtos.html',
  styleUrl: './produtos.css'
})
export class Produtos implements OnInit {
  produtos = signal<Produto[]>([]);
  erro = signal('');

  novoProduto: Produto = { codigo: '', descricao: '', saldo: 0 };

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
    this.produtosService.criar(this.novoProduto).subscribe({
      next: () => {
        this.novoProduto = { codigo: '', descricao: '', saldo: 0 };
        this.carregar();
      },
      error: (err) => this.erro.set('Falha ao criar produto: ' + err.message)
    });
  }
}
