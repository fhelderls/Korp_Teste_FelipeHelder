# Korp Teste Felipe Helder

Sistema de emissão de notas fiscais construído em arquitetura de microsserviços, como parte do teste técnico da Korp (trilha Go).

## Sobre o projeto

O sistema é dividido em dois microsserviços independentes que se comunicam via HTTP, mais um frontend Angular:

- **estoque-service**: gerencia produtos (código, descrição, saldo, preço) e reservas de estoque.
- **faturamento-service**: gerencia notas fiscais, orquestra a emissão chamando o estoque-service, gera PDF das notas e um resumo de insights de vendas com IA.
- **frontend**: interface Angular para cadastrar/excluir produtos e emitir notas fiscais com múltiplos itens.

Cada microsserviço tem seu próprio banco de dados PostgreSQL (`estoque` e `faturamento`), rodando na mesma instância do Postgres mas isolados em bancos separados.

## Funcionalidades

- CRUD de produtos (criar, listar, excluir), com preço.
- Emissão de nota fiscal com múltiplos produtos por nota.
- Download da nota fiscal em PDF.
- Resumo de insights de vendas gerado por IA (quantidade/valor vendido por produto e por cliente, no estilo do painel de insights da Korp), a partir de dados reais das notas emitidas.
- Cenário de falha com recuperação automática (ver seção abaixo).

## Arquitetura e tratamento de falha

A emissão de uma nota fiscal segue um fluxo de três etapas, inspirado no padrão Saga:

1. O `faturamento-service` grava a nota com status `pendente`.
2. Chama `POST /reservas` no `estoque-service`, que valida o saldo e desconta o estoque dentro de uma transação com `SELECT ... FOR UPDATE` (trava a linha do produto até a transação terminar, evitando que duas reservas concorrentes deixem o saldo inconsistente).
3. Se a reserva deu certo, chama `POST /reservas/{chave}/confirmar`. Se der certo, a nota vira `emitida`.

Se o `estoque-service` estiver fora do ar ou recusar a reserva (saldo insuficiente), a nota é marcada como `falha` e o erro é devolvido ao usuário de forma clara, sem deixar dado inconsistente.

Se a reserva foi feita mas a confirmação falhar depois (por exemplo, o `estoque-service` cair nesse intervalo), o `faturamento-service` chama `POST /reservas/{chave}/cancelar`, que devolve o saldo reservado. Essa é a ação de compensação do padrão Saga.

Todo o fluxo é idempotente pela `chave` da nota: reenviar a mesma requisição para uma nota que já está `emitida` simplesmente devolve a nota existente, e uma nota que ficou `falha` pode ser reprocessada com segurança sem duplicar a reserva.

As chamadas HTTP do `faturamento-service` para o `estoque-service` (reservar/confirmar/cancelar) têm retry com backoff exponencial (3 tentativas, 200ms/400ms/800ms, timeout de 2s por tentativa) para falhas de rede — uma indisponibilidade momentânea não precisa virar falha na hora. Recusas de regra de negócio (ex: saldo insuficiente) não são reprocessadas, já que tentar de novo não muda o resultado.

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

## Testando o cenário de falha manualmente

```bash
# derruba o estoque-service
docker compose stop estoque-service

# tenta emitir uma nota (vai falhar apos as tentativas de retry, nota fica "falha")
curl -X POST http://localhost:8082/notas -H "Content-Type: application/json" -d '{"chave":"nf-teste","cliente":"Cliente","itens":[{"produto_codigo":"P001","quantidade":1}]}'

# sobe o estoque-service de novo
docker compose start estoque-service

# reenvia a mesma nota (agora emite com sucesso)
curl -X POST http://localhost:8082/notas -H "Content-Type: application/json" -d '{"chave":"nf-teste","cliente":"Cliente","itens":[{"produto_codigo":"P001","quantidade":1}]}'
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
- **RxJS**: o `HttpClient` do Angular devolve `Observable` em cada chamada (`listar`, `criar`, `emitir`, `resumo`). Os componentes assinam esses Observables com `.subscribe({ next, error })`, tratando separadamente o caminho de sucesso e o de erro (é assim que a mensagem de falha do backend chega até a tela quando a emissão de uma nota falha).
- **Signals**: o projeto foi gerado pelo Angular CLI (v21) sem `zone.js`, ou seja, roda em modo zoneless. Nesse modo, atribuir direto a uma propriedade normal da classe dentro de um `.subscribe()` não notifica a detecção de mudanças. Por isso o estado dos componentes (`produtos`, `notas`, `erro`, `resumoIA`, `itens`) é guardado em `signal()`, que é a primitiva reativa que funciona corretamente sem `zone.js`.
- **Outras bibliotecas**: `FormsModule` (`[(ngModel)]`) para os formulários, com two-way binding simples em vez de Reactive Forms; `CurrencyPipe` para formatar preços.

### Backend (Go)

- **Gerenciamento de dependências**: Go Modules (`go.mod`/`go.sum`), nativo da linguagem. Cada microsserviço é um módulo Go independente, com suas próprias dependências.
- **Frameworks e bibliotecas**: nenhum framework web externo. As rotas HTTP usam `net/http` da biblioteca padrão, aproveitando o roteador nativo (`http.ServeMux`) com suporte a método + padrão de caminho (`"POST /reservas/{chave}/confirmar"`), disponível desde o Go 1.22. Para o banco, o driver `github.com/jackc/pgx/v5/stdlib`, usado através da interface padrão `database/sql`. Para o PDF, `github.com/go-pdf/fpdf`.
- **Tratamento de erros**: erros são retornados como valores (padrão idiomático do Go), envolvidos com contexto usando `fmt.Errorf` e `%w` a cada camada. Nos handlers HTTP, cada erro vira uma resposta com o status apropriado (`400` para requisição inválida, `404` para recurso não encontrado, `409` para conflito de regra de negócio, `503` para falha de comunicação entre serviços), sempre com uma mensagem legível no corpo, nunca um erro genérico.

### Concorrência

O `Reservar` do `estoque-service` roda dentro de uma transação com `SELECT ... FOR UPDATE` na linha do produto. Isso trava a linha durante a transação, garantindo que duas reservas concorrentes para o mesmo produto sejam processadas em sequência, não em paralelo, evitando que o saldo fique inconsistente (ex: duas reservas lendo o mesmo saldo antes de qualquer uma descontar).

### Idempotência

Tanto a reserva de estoque quanto a nota fiscal usam uma `chave` como identificador único (chave primária no banco). Reenviar a mesma requisição de emissão com a mesma chave não duplica nada: se a nota já está `emitida`, o sistema devolve ela como está; se está `falha`, o sistema tenta o fluxo de novo a partir do mesmo registro, sem recriar a nota do zero.

### Uso de IA

O uso de IA aparece em duas frentes neste projeto:

1. **Como funcionalidade do produto**: o endpoint `GET /notas/resumo` agrega os dados reais das notas emitidas (quantidade e valor vendido por produto e por cliente) e envia esse resumo para a API da Anthropic (modelo Claude Haiku), que devolve um texto em linguagem natural destacando os principais números, no estilo do painel de insights de vendas da Korp. A chave de API fica só no backend, nunca é exposta no frontend.
2. **Como ferramenta auxiliar no desenvolvimento**: o Claude Code foi usado ao longo de todo o processo para explicar conceitos novos (Go, Angular, Docker) conforme surgiam, revisar o código digitado manualmente antes de cada commit, e apoiar no diagnóstico de problemas de ambiente. Todo o código foi digitado e compreendido pelo candidato; a IA funcionou como par de programação e fonte de explicação, não como gerador autônomo de solução.
