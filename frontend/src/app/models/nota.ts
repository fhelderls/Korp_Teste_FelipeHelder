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
