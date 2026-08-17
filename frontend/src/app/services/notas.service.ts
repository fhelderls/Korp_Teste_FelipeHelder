import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { NotaFiscal, CriarRequest } from '../models/nota';

@Injectable({
  providedIn: 'root'
})
export class NotasService {
  private readonly baseUrl = 'http://localhost:8082';

  constructor(private http: HttpClient) {}

  listar(): Observable<NotaFiscal[]> {
    return this.http.get<NotaFiscal[]>(`${this.baseUrl}/notas`);
  }

  criar(nota: CriarRequest): Observable<NotaFiscal> {
    return this.http.post<NotaFiscal>(`${this.baseUrl}/notas`, nota);
  }

  imprimir(chave: string): Observable<NotaFiscal> {
    return this.http.post<NotaFiscal>(`${this.baseUrl}/notas/${chave}/imprimir`, {});
  }

  pdfUrl(chave: string): string {
    return `${this.baseUrl}/notas/${chave}/pdf`;
  }

  resumo(): Observable<{ resumo: string }> {
    return this.http.get<{ resumo: string }>(`${this.baseUrl}/notas/resumo`);
  }
}
