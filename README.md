# Korp Teste Felipe Helder

Sistema de emissão de notas fiscais construído em arquitetura de microsserviços, como parte do teste técnico da Korp (trilha Go).

## Sobre o projeto

O sistema é dividido em dois microsserviços independentes que se comunicam via HTTP, mais um frontend Angular:

- **estoque-service**: gerencia produtos (código, descrição, saldo, preço) e reservas de estoque.
- **faturamento-service**: gerencia notas fiscais, orquestra a emissão chamando o estoque-service, gera PDF das notas e um resumo de insights de vendas com IA.
- **frontend**: interface Angular para cadastrar/editar/excluir produtos e cadastrar/emitir notas fiscais com múltiplos itens.

Cada microsserviço tem seu próprio banco de dados PostgreSQL (`estoque` e `faturamento`), rodando na mesma instância do Postgres mas isolados em bancos separados.

## Funcionalidades

- CRUD de produtos (criar, listar, editar, excluir), com preço. Código do produto (`PROD-001`, `PROD-002`...) gerado automaticamente, nunca escolhido pelo usuário. A edição serve principalmente para reajustar o saldo em estoque sem precisar excluir e recriar o produto.
- Saldo de estoque em três partes: saldo total, saldo reservado (soma das quantidades de todas as notas ainda `Aberta` para aquele produto) e saldo disponível (`total - reservado`), calculado em tempo real e exposto pela API de produtos. O saldo disponível pode ficar **negativo** quando notas abertas, juntas, comprometem mais estoque do que existe — é o sinal visível de que há uma disputa por aquele produto ainda não resolvida.
- Cadastro de nota fiscal com múltiplos produtos (escolhidos pela descrição, não pelo código). Número da nota (`NF-001`, `NF-002`...) gerado automaticamente. Nasce com status **Aberta**, data de abertura registrada, e já **registra a reserva** dos itens nesse momento (conta para o saldo reservado do produto). A descrição e o preço de cada item são gravados na própria nota nesse momento (um retrato do produto na hora da compra) — editar o preço de um produto depois, ou até excluí-lo, não muda o histórico de notas já criadas.
- Emissão de nota fiscal: botão dedicado, com indicador de processamento enquanto roda. Confirma a reserva feita na criação — é aqui que o saldo de verdade é conferido e debitado, resolvendo a disputa entre notas concorrentes — e muda o status para **Fechada**, gravando a data de emissão. Uma nota `Fechada` não pode ser emitida de novo; se a emissão falhar (ex: outra nota concorrente já consumiu o saldo), a nota continua `Aberta` com a reserva ainda pendente, e pode ser emitida de novo depois.
- Cancelamento de nota fiscal: só disponível enquanto a nota está `Aberta` (uma nota `Fechada` é definitiva). Libera a reserva de estoque correspondente e remove a nota — pensado para desistir de uma nota que nunca vai ser emitida, sem deixar estoque preso indefinidamente.
- Download da nota fiscal em PDF (com nome e preço dos produtos, não o código, e as datas de abertura e emissão).
- Resumo de insights de vendas gerado por IA (quantidade/valor vendido por produto e por cliente, no estilo do painel de insights da Korp), a partir de dados reais das notas fechadas.
- Relatório de faturamento em PDF (estilo dashboard): cabeçalho com data de geração, cartões de indicadores (notas fechadas/abertas, ticket médio, clientes atendidos), destaques de produto e cliente com maior faturamento, gráficos de pizza com a participação percentual no faturamento por produto e por cliente, e gráficos de barra com a quantidade vendida por produto e por cliente — tudo a partir dos mesmos dados reais usados no resumo de IA.
- Cenário de falha com recuperação (ver seção abaixo).

## Arquitetura e tratamento de falha

Criar uma nota fiscal (`POST /notas`) e emiti-la (`POST /notas/{chave}/imprimir`) são duas etapas inspiradas no padrão Saga. A ideia central: **reservar é barato e sempre aceito, confirmar é onde a disputa por estoque é resolvida de verdade.**

1. **Criar**: o `faturamento-service` grava a nota com número sequencial gerado automaticamente e status `Aberta`, e imediatamente chama `POST /reservas` no `estoque-service`, que só registra a intenção de uso (itens e quantidades) com status `pendente` — **sem conferir nem descontar saldo**. Por isso a criação praticamente nunca falha por regra de negócio: só falha se o `estoque-service` estiver mesmo fora do ar, e nesse caso a criação da nota inteira é desfeita (não fica nota `Aberta` sem reserva correspondente).
2. **Emitir**: confere que a nota está `Aberta` (uma nota `Fechada` não pode ser emitida de novo) e chama `POST /reservas/{chave}/confirmar`. É só agora que o `estoque-service` de fato verifica o saldo e o desconta, dentro de uma transação com `SELECT ... FOR UPDATE` (trava a linha do produto até a transação terminar). Se der certo, a nota vira `Fechada` e grava a data de emissão.

Como reservar nunca verifica saldo, **o saldo reservado de um produto pode passar do saldo total** quando várias notas `Aberta` disputam o mesmo item — o saldo disponível (`saldo - reservado`) fica negativo, sinalizando a disputa. Ela só é resolvida quando as notas tentam ser emitidas: a primeira a confirmar leva o saldo; as demais falham na emissão com "estoque insuficiente" e continuam `Aberta`, podendo tentar de novo depois (se o saldo for reposto, ou a nota concorrente for descartada).

Se o `estoque-service` estiver fora do ar ou recusar a confirmação por falta de saldo, a nota **continua `Aberta`** (com a reserva ainda pendente, sem data de emissão), e o erro é devolvido ao usuário de forma clara. Não existe um status de "falha" separado: uma nota que não foi emitida com sucesso simplesmente segue `Aberta`, e o usuário pode clicar em Emitir de novo quando quiser — essa é a própria forma de reprocessar, sem precisar de uma ação especial.

Uma nota `Aberta` também pode ser **cancelada** (`DELETE /notas/{chave}`) pelo usuário, se ele desistir dela: chama `POST /reservas/{chave}/cancelar` no `estoque-service` (que libera a reserva, sem precisar devolver saldo nenhum, já que reservar nunca descontou nada) e remove a nota. É a terceira ação do padrão Saga, ao lado de confirmar — sem ela, uma nota `Aberta` abandonada deixaria estoque preso pra sempre.

As chamadas HTTP do `faturamento-service` para o `estoque-service` (reservar/confirmar) têm retry com backoff exponencial (3 tentativas, 200ms/400ms/800ms, timeout de 2s por tentativa) para falhas de rede — uma indisponibilidade momentânea não precisa virar falha na hora. Recusas de regra de negócio na confirmação (saldo insuficiente) não são reprocessadas automaticamente, já que tentar de novo na hora não muda o resultado.

## Como rodar

**1. Configure a chave de IA** (opcional, só necessário para o resumo de insights): copie `.env.example` para `.env` e preencha `ANTHROPIC_API_KEY` com uma chave da [Anthropic Console](https://console.anthropic.com/settings/keys). Sem isso, o resto do sistema funciona normalmente, só o resumo com IA não vai responder.

**2. Suba tudo com Docker Compose**, na raiz do projeto:

```bash
docker compose up --build
```

Isso sobe:
- Postgres na porta `5435`
- estoque-service na porta `8083`
- faturamento-service na porta `8082`
- frontend na porta `4200`

Acesse `http://localhost:4200` para usar a interface.

**3. (Opcional) Popule com dados de exemplo**, para testar a interface, o resumo de IA e o relatório em PDF sem cadastrar tudo manualmente. Com os containers no ar, rode no Git Bash:

```bash
bash scripts/seed.sh
```

O script cria 15 produtos e 18 notas fiscais via API (usando os mesmos endpoints públicos do sistema, sem acesso direto ao banco): a maioria fechada com dados variados de cliente/produto, duas propositalmente deixadas "Aberta", e uma criada com quantidade maior que o saldo (a criação funciona normalmente, só a tentativa de emissão falha de propósito, ficando "Aberta"), para exercitar o cenário de erro.

## Testando o cenário de falha manualmente

```bash
# cria uma nota (fica "Aberta")
curl -X POST http://localhost:8082/notas -H "Content-Type: application/json" -d '{"cliente":"Cliente","itens":[{"produto_codigo":"PROD-001","quantidade":1}]}'
# anote o "chave" da resposta (ex: "NF-003")

# derruba o estoque-service
docker compose stop estoque-service

# tenta emitir (vai falhar apos as tentativas de retry, nota continua "Aberta")
curl -X POST http://localhost:8082/notas/NF-003/imprimir

# sobe o estoque-service de novo
docker compose start estoque-service

# emite de novo (agora fecha com sucesso)
curl -X POST http://localhost:8082/notas/NF-003/imprimir
```

## Testes automatizados

```bash
cd estoque-service && go test ./... -v
cd faturamento-service && go test ./... -v
```

São testes de integração, rodando contra o Postgres real (não usam mocks de banco), cobrindo desconto de saldo, saldo insuficiente, confirmação idempotente, cancelamento com devolução de saldo, e o caso de nota não encontrada.

Um workflow do GitHub Actions (`.github/workflows/ci.yml`) roda essa mesma suíte automaticamente a cada push/PR, junto com o build do frontend Angular.

## Detalhamento técnico

### Frontend (Angular)

- **Lifecycle hooks**: `ngOnInit` é usado nos dois componentes principais (`Produtos` e `Notas`) para disparar a busca inicial de dados assim que o componente é criado.
- **RxJS**: o `HttpClient` do Angular devolve `Observable` em cada chamada (`listar`, `criar`, `emitir`, `resumo`). Os componentes assinam esses Observables com `.subscribe({ next, error })`, tratando separadamente o caminho de sucesso e o de erro (é assim que a mensagem de falha do backend chega até a tela quando a emissão de uma nota falha, e que o indicador de "Processando..." é controlado durante a chamada).
- **Signals**: o projeto foi gerado pelo Angular CLI (v21) sem `zone.js`, ou seja, roda em modo zoneless. Nesse modo, atribuir direto a uma propriedade normal da classe dentro de um `.subscribe()` não notifica a detecção de mudanças. Por isso o estado dos componentes (`produtos`, `notas`, `erro`, `resumoIA`, `itens`, `emitindo`) é guardado em `signal()`, que é a primitiva reativa que funciona corretamente sem `zone.js`. A lista de produtos vive como `signal` compartilhado no `ProdutosService` (não duplicada em cada componente), para telas diferentes ficarem sincronizadas sem precisar recarregar a página.
- **Outras bibliotecas**: `FormsModule` (`[(ngModel)]`) para os formulários, com two-way binding simples em vez de Reactive Forms; `CurrencyPipe` para formatar preços. Os itens da nota usam um `<select>` com a descrição dos produtos (carregados do `estoque-service`), não o código.
- **Componentes visuais**: nenhuma biblioteca de UI (sem Angular Material, Bootstrap etc.). CSS puro, escrito à mão, com um pequeno design system compartilhado em `styles.css` (cards, tabelas, badges de status, botões).

### Backend (Go)

- **Gerenciamento de dependências**: Go Modules (`go.mod`/`go.sum`), nativo da linguagem. Cada microsserviço é um módulo Go independente, com suas próprias dependências.
- **Frameworks e bibliotecas**: nenhum framework web externo. As rotas HTTP usam `net/http` da biblioteca padrão, aproveitando o roteador nativo (`http.ServeMux`) com suporte a método + padrão de caminho (`"POST /notas/{chave}/imprimir"`), disponível desde o Go 1.22. Para o banco, o driver `github.com/jackc/pgx/v5/stdlib`, usado através da interface padrão `database/sql`. Para o PDF, `github.com/go-pdf/fpdf`.
- **Tratamento de erros**: erros são retornados como valores (padrão idiomático do Go), envolvidos com contexto usando `fmt.Errorf` e `%w` a cada camada. Nos handlers HTTP, cada erro vira uma resposta com o status apropriado (`400` para requisição inválida, `404` para recurso não encontrado, `409` para conflito de regra de negócio como tentar emitir de novo uma nota já fechada, `503` para falha de comunicação entre serviços), sempre com uma mensagem legível no corpo, nunca um erro genérico.

### Concorrência

O `Confirmar` do `estoque-service` roda dentro de uma transação com `SELECT ... FOR UPDATE` na linha do produto. Isso trava a linha durante a transação, garantindo que duas confirmações concorrentes para o mesmo produto sejam processadas em sequência, não em paralelo, evitando que o saldo fique inconsistente (ex: duas notas sendo emitidas ao mesmo tempo disputando o último item em estoque — uma delas espera a outra terminar antes de conferir o saldo).

### Idempotência

Tanto a reserva de estoque quanto a nota fiscal usam uma `chave`/número como identificador único (chave primária no banco, gerada automaticamente e sequencial). `POST /notas` sempre cria uma nota nova e registra a reserva uma única vez; emitir é uma ação separada (`POST /notas/{chave}/imprimir`) que só confirma a reserva já existente, nunca registra outra: só funciona em notas `Aberta`, e uma nota `Fechada` recusa ser emitida de novo (evita confirmar/debitar o estoque duas vezes pela mesma nota).

### Uso de IA

O uso de IA aparece em duas frentes neste projeto:

1. **Como funcionalidade do produto**: o endpoint `GET /notas/resumo` agrega os dados reais das notas fechadas (quantidade e valor vendido por produto e por cliente) e envia esse resumo para a API da Anthropic (modelo Claude Haiku), que devolve um texto em linguagem natural destacando os principais números, no estilo do painel de insights de vendas da Korp. A chave de API fica só no backend, nunca é exposta no frontend.
2. **Como ferramenta auxiliar no desenvolvimento**: o Claude Code foi usado ao longo de todo o processo para explicar conceitos novos (Go, Angular, Docker) conforme surgiam, revisar o código digitado manualmente antes de cada commit, e apoiar no diagnóstico de problemas de ambiente. Todo o código foi digitado e compreendido pelo candidato; a IA funcionou como par de programação e fonte de explicação, não como gerador autônomo de solução.
