import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Produto } from '../models/produto';

@Injectable({
  providedIn: 'root'
})
export class ProdutosService {
  private readonly baseUrl = 'http://localhost:8083';

  // Lista compartilhada por todos os componentes que injetam este service,
  // pra tela de Notas sempre ver os produtos mais recentes (inclusive os
  // recem-criados na tela de Produtos) sem precisar recarregar a pagina.
  produtos = signal<Produto[]>([]);

  constructor(private http: HttpClient) {}

  carregar(): void {
    this.http.get<Produto[]>(`${this.baseUrl}/produtos`).subscribe({
      next: (produtos) => this.produtos.set(produtos)
    });
  }

  criar(produto: Produto): Observable<Produto> {
    return this.http.post<Produto>(`${this.baseUrl}/produtos`, produto);
  }

  atualizar(codigo: string, produto: Produto): Observable<Produto> {
    return this.http.put<Produto>(`${this.baseUrl}/produtos/${codigo}`, produto);
  }

  deletar(codigo: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/produtos/${codigo}`);
  }
}
