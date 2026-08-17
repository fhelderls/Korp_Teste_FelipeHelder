export interface ItemNota {
  produto_codigo: string;
  quantidade: number;
}

export interface NotaFiscal {
  chave: string;
  cliente: string;
  itens: ItemNota[];
  status: string;
}

// CriarRequest e o que o cliente envia para cadastrar uma nota nova - a
// chave (numero da nota, ex: NF-001) e gerada pelo backend, nunca pelo
// cliente. A nota nasce como "Aberta"; imprimir e uma acao separada.
export interface CriarRequest {
  cliente: string;
  itens: ItemNota[];
}
