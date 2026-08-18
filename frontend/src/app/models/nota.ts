export interface ItemNota {
  produto_codigo: string;
  quantidade: number;
  // descricao e preco_unitario sao preenchidos pelo backend na criacao da
  // nota (retrato do produto naquele momento) - o cliente nao envia isso
  // ao criar, só recebe de volta depois.
  descricao?: string;
  preco_unitario?: number;
}

export interface NotaFiscal {
  chave: string;
  cliente: string;
  itens: ItemNota[];
  status: string;
  data_abertura: string;
  data_emissao: string | null;
}

// CriarRequest e o que o cliente envia para cadastrar uma nota nova - a
// chave (numero da nota, ex: NF-001) e gerada pelo backend, nunca pelo
// cliente. A nota nasce como "Aberta"; imprimir e uma acao separada.
export interface CriarRequest {
  cliente: string;
  itens: ItemNota[];
}
