#!/bin/bash
set -e

ESTOQUE=http://localhost:8083
FATURAMENTO=http://localhost:8082

criar_produto() {
  # $1 = descricao, $2 = saldo, $3 = preco
  resposta=$(curl.exe -s -X POST "$ESTOQUE/produtos" -H "Content-Type: application/json" -d "{\"descricao\":\"$1\",\"saldo\":$2,\"preco\":$3}")
  echo "$resposta" | python3 -c "import json,sys; print(json.load(sys.stdin)['codigo'])"
}

criar_nota() {
  # $1 = cliente, $2 = itens (json array) -> devolve a chave gerada
  resposta=$(curl.exe -s -X POST "$FATURAMENTO/notas" -H "Content-Type: application/json" -d "{\"cliente\":\"$1\",\"itens\":$2}")
  echo "$resposta" | python3 -c "import json,sys; print(json.load(sys.stdin)['chave'])"
}

imprimir_nota() {
  # $1 = chave
  curl.exe -s -X POST "$FATURAMENTO/notas/$1/imprimir" > /dev/null
}

echo "Criando produtos..."
CANETA_AZ=$(criar_produto "Caneta esferografica azul" 500 2.50)
CANETA_PT=$(criar_produto "Caneta esferografica preta" 480 2.50)
CADERNO_100=$(criar_produto "Caderno universitario 100 folhas" 200 18.90)
CADERNO_200=$(criar_produto "Caderno universitario 200 folhas" 150 32.90)
LAPIS_HB=$(criar_produto "Lapis grafite HB" 600 1.20)
BORRACHA=$(criar_produto "Borracha branca" 400 1.80)
MOCHILA=$(criar_produto "Mochila escolar" 60 89.90)
ESTOJO=$(criar_produto "Estojo escolar duplo" 90 24.90)
MARCA_TEXTO=$(criar_produto "Marca texto fluorescente" 250 4.50)
COLA=$(criar_produto "Cola bastao 20g" 300 3.20)
TESOURA=$(criar_produto "Tesoura escolar sem ponta" 120 6.90)
REGUA=$(criar_produto "Regua 30cm" 350 2.90)
APONTADOR=$(criar_produto "Apontador com deposito" 280 3.50)
FOLHA_A4=$(criar_produto "Pacote papel sulfite A4 500fls" 100 24.90)
CANETA_GEL=$(criar_produto "Caneta gel colorida kit 6 cores" 80 15.90)

echo "Produtos criados (15)."

echo "Criando e imprimindo notas fiscais..."

nota_fechada() {
  chave=$(criar_nota "$1" "$2")
  imprimir_nota "$chave"
}

nota_fechada "Papelaria Bom Preco" "[{\"produto_codigo\":\"$CANETA_AZ\",\"quantidade\":50},{\"produto_codigo\":\"$CANETA_PT\",\"quantidade\":40}]"
nota_fechada "Escola Municipal Sao Jose" "[{\"produto_codigo\":\"$CADERNO_100\",\"quantidade\":30},{\"produto_codigo\":\"$LAPIS_HB\",\"quantidade\":60}]"
nota_fechada "Livraria Central" "[{\"produto_codigo\":\"$CADERNO_200\",\"quantidade\":15}]"
nota_fechada "Distribuidora ABC Papelaria" "[{\"produto_codigo\":\"$MOCHILA\",\"quantidade\":10},{\"produto_codigo\":\"$ESTOJO\",\"quantidade\":20}]"
nota_fechada "Comercio Silva e Filhos" "[{\"produto_codigo\":\"$MARCA_TEXTO\",\"quantidade\":25},{\"produto_codigo\":\"$COLA\",\"quantidade\":15}]"
nota_fechada "Mercadinho do Bairro" "[{\"produto_codigo\":\"$CANETA_AZ\",\"quantidade\":20}]"
nota_fechada "Papelaria Universitaria" "[{\"produto_codigo\":\"$CADERNO_100\",\"quantidade\":40},{\"produto_codigo\":\"$CANETA_GEL\",\"quantidade\":10}]"
nota_fechada "Kids Store Materiais" "[{\"produto_codigo\":\"$MOCHILA\",\"quantidade\":8},{\"produto_codigo\":\"$ESTOJO\",\"quantidade\":8}]"
nota_fechada "Papelaria Bom Preco" "[{\"produto_codigo\":\"$TESOURA\",\"quantidade\":12},{\"produto_codigo\":\"$REGUA\",\"quantidade\":30}]"
nota_fechada "Escola Municipal Sao Jose" "[{\"produto_codigo\":\"$APONTADOR\",\"quantidade\":50},{\"produto_codigo\":\"$BORRACHA\",\"quantidade\":50}]"
nota_fechada "Livraria Central" "[{\"produto_codigo\":\"$FOLHA_A4\",\"quantidade\":20}]"
nota_fechada "Distribuidora ABC Papelaria" "[{\"produto_codigo\":\"$CANETA_AZ\",\"quantidade\":80},{\"produto_codigo\":\"$CANETA_PT\",\"quantidade\":70}]"
nota_fechada "Comercio Silva e Filhos" "[{\"produto_codigo\":\"$CADERNO_200\",\"quantidade\":10},{\"produto_codigo\":\"$CANETA_GEL\",\"quantidade\":15}]"
nota_fechada "Papelaria Universitaria" "[{\"produto_codigo\":\"$MARCA_TEXTO\",\"quantidade\":18}]"
nota_fechada "Mercadinho do Bairro" "[{\"produto_codigo\":\"$LAPIS_HB\",\"quantidade\":40},{\"produto_codigo\":\"$BORRACHA\",\"quantidade\":30}]"
nota_fechada "Kids Store Materiais" "[{\"produto_codigo\":\"$CADERNO_100\",\"quantidade\":25},{\"produto_codigo\":\"$ESTOJO\",\"quantidade\":12}]"
nota_fechada "Papelaria Bom Preco" "[{\"produto_codigo\":\"$FOLHA_A4\",\"quantidade\":15},{\"produto_codigo\":\"$COLA\",\"quantidade\":20}]"
nota_fechada "Livraria Central" "[{\"produto_codigo\":\"$CANETA_GEL\",\"quantidade\":22}]"

# duas notas ficam "Aberta" de proposito, sem imprimir, pra testar o botao ao vivo
criar_nota "Escola Municipal Sao Jose" "[{\"produto_codigo\":\"$REGUA\",\"quantidade\":45},{\"produto_codigo\":\"$APONTADOR\",\"quantidade\":45}]" > /dev/null
criar_nota "Distribuidora ABC Papelaria" "[{\"produto_codigo\":\"$MOCHILA\",\"quantidade\":5},{\"produto_codigo\":\"$TESOURA\",\"quantidade\":10}]" > /dev/null

# uma nota criada com quantidade maior que o saldo: a CRIACAO funciona
# normalmente (so registra a intencao, sem checar saldo), mas a tentativa
# de EMISSAO falha (e ai que o saldo de verdade e conferido) - a nota fica
# "Aberta", pra testar o cenario de erro ao vivo
chave_falha=$(criar_nota "Comercio Silva e Filhos" "[{\"produto_codigo\":\"$MOCHILA\",\"quantidade\":9999}]")
imprimir_nota "$chave_falha" || true

echo "Seed concluido."
