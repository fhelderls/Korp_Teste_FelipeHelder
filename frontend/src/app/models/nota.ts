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

// EmitirRequest e o que o cliente envia para criar uma nota nova - a chave
// (numero da nota, ex: NF-001) e gerada pelo backend, nunca pelo cliente.
export interface EmitirRequest {
  cliente: string;
  itens: ItemNota[];
}
