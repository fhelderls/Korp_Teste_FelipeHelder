import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { NotaFiscal, EmitirRequest } from '../models/nota';

@Injectable({
  providedIn: 'root'
})
export class NotasService {
  private readonly baseUrl = 'http://localhost:8082';

  constructor(private http: HttpClient) {}

  listar(): Observable<NotaFiscal[]> {
    return this.http.get<NotaFiscal[]>(`${this.baseUrl}/notas`);
  }

  emitir(nota: EmitirRequest): Observable<NotaFiscal> {
    return this.http.post<NotaFiscal>(`${this.baseUrl}/notas`, nota);
  }

  reprocessar(chave: string): Observable<NotaFiscal> {
    return this.http.post<NotaFiscal>(`${this.baseUrl}/notas/${chave}/reprocessar`, {});
  }

  resumo(): Observable<{ resumo: string }> {
    return this.http.get<{ resumo: string }>(`${this.baseUrl}/notas/resumo`);
  }
}
