import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { NotasService } from '../services/notas.service';
import { NotaFiscal } from '../models/nota';

@Component({
  selector: 'app-notas',
  imports: [FormsModule],
  templateUrl: './notas.html',
  styleUrl: './notas.css'
})
export class Notas implements OnInit {
  notas = signal<NotaFiscal[]>([]);
  erro = signal('');

  chave = '';
  cliente = '';
  produtoCodigo = '';
  quantidade = 1;

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

  emitir(): void {
    this.erro.set('');
    const nota: NotaFiscal = {
      chave: this.chave,
      cliente: this.cliente,
      itens: [{ produto_codigo: this.produtoCodigo, quantidade: this.quantidade }],
      status: ''
    };

    this.notasService.emitir(nota).subscribe({
      next: () => {
        this.chave = '';
        this.cliente = '';
        this.produtoCodigo = '';
        this.quantidade = 1;
        this.carregar();
      },
      error: (err) => {
        this.erro.set(err.error ?? 'Falha ao emitir nota fiscal');
        this.carregar();
      }
    });
  }
}
