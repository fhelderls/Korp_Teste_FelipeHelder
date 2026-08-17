# Korp Teste Felipe Helder

Sistema de emissão de notas fiscais construído em arquitetura de microsserviços, como parte do teste técnico da Korp (trilha Go).

## Sobre o projeto

O sistema é dividido em dois microsserviços independentes que se comunicam via HTTP, mais um frontend Angular:

- **estoque-service**: gerencia produtos e reservas de estoque.
- **faturamento-service**: gerencia notas fiscais e orquestra a emissão, chamando o estoque-service.
- **frontend**: interface Angular para cadastrar produtos e emitir notas fiscais.

Cada microsserviço tem seu próprio banco de dados PostgreSQL (`estoque` e `faturamento`), rodando na mesma instância do Postgres mas isolados em bancos separados.

## Arquitetura e tratamento de falha

A emissão de uma nota fiscal segue um fluxo de três etapas, inspirado no padrão Saga:

1. O `faturamento-service` grava a nota com status `pendente`.
2. Chama `POST /reservas` no `estoque-service`, que valida o saldo e desconta o estoque dentro de uma transação com `SELECT ... FOR UPDATE` (trava a linha do produto até a transação terminar, evitando que duas reservas concorrentes deixem o saldo inconsistente).
3. Se a reserva deu certo, chama `POST /reservas/{chave}/confirmar`. Se der certo, a nota vira `emitida`.

Se o `estoque-service` estiver fora do ar ou recusar a reserva (saldo insuficiente), a nota é marcada como `falha` e o erro é devolvido ao usuário de forma clara, sem deixar dado inconsistente.

Se a reserva foi feita mas a confirmação falhar depois (por exemplo, o `estoque-service` cair nesse intervalo), o `faturamento-service` chama `POST /reservas/{chave}/cancelar`, que devolve o saldo reservado. Essa é a ação de compensação do padrão Saga.

Todo o fluxo é idempotente pela `chave` da nota: reenviar a mesma requisição para uma nota que já está `emitida` simplesmente devolve a nota existente, e uma nota que ficou `falha` pode ser reprocessada com segurança sem duplicar a reserva.

## Como rodar

Com Docker e Docker Compose instalados, na raiz do projeto:

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

# tenta emitir uma nota (vai falhar com erro claro, nota fica "falha")
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

## Detalhamento técnico

### Frontend (Angular)

- **Lifecycle hooks**: `ngOnInit` é usado nos dois componentes principais (`Produtos` e `Notas`) para disparar a busca inicial de dados assim que o componente é criado.
- **RxJS**: o `HttpClient` do Angular devolve `Observable` em cada chamada (`listar`, `criar`, `emitir`). Os componentes assinam esses Observables com `.subscribe({ next, error })`, tratando separadamente o caminho de sucesso e o de erro (é assim que a mensagem de falha do backend chega até a tela quando a emissão de uma nota falha).
- **Signals**: o projeto foi gerado pelo Angular CLI (v21) sem `zone.js`, ou seja, roda em modo zoneless. Nesse modo, atribuir direto a uma propriedade normal da classe dentro de um `.subscribe()` não notifica a detecção de mudanças. Por isso o estado dos componentes (`produtos`, `notas`, `erro`) é guardado em `signal()`, que é a primitiva reativa que funciona corretamente sem `zone.js`.
- **Outras bibliotecas**: `FormsModule` (`[(ngModel)]`) para os formulários de criação de produto e emissão de nota, com two-way binding simples em vez de Reactive Forms.

### Backend (Go)

- **Gerenciamento de dependências**: Go Modules (`go.mod`/`go.sum`), nativo da linguagem. Cada microsserviço é um módulo Go independente, com suas próprias dependências.
- **Frameworks e bibliotecas**: nenhum framework web externo. As rotas HTTP usam `net/http` da biblioteca padrão, aproveitando o roteador nativo (`http.ServeMux`) com suporte a método + padrão de caminho (`"POST /reservas/{chave}/confirmar"`), disponível desde o Go 1.22. Para o banco, o driver `github.com/jackc/pgx/v5/stdlib`, usado através da interface padrão `database/sql`.
- **Tratamento de erros**: erros são retornados como valores (padrão idiomático do Go), envolvidos com contexto usando `fmt.Errorf` e `%w` a cada camada. Nos handlers HTTP, cada erro vira uma resposta com o status apropriado (`400` para requisição inválida, `409` para conflito de regra de negócio, `503` para falha de comunicação entre serviços), sempre com uma mensagem legível no corpo, nunca um erro genérico.

### Concorrência

O `Reservar` do `estoque-service` roda dentro de uma transação com `SELECT ... FOR UPDATE` na linha do produto. Isso trava a linha durante a transação, garantindo que duas reservas concorrentes para o mesmo produto sejam processadas em sequência, não em paralelo, evitando que o saldo fique inconsistente (ex: duas reservas lendo o mesmo saldo antes de qualquer uma descontar).

### Idempotência

Tanto a reserva de estoque quanto a nota fiscal usam uma `chave` como identificador único (chave primária no banco). Reenviar a mesma requisição de emissão com a mesma chave não duplica nada: se a nota já está `emitida`, o sistema devolve ela como está; se está `falha`, o sistema tenta o fluxo de novo a partir do mesmo registro, sem recriar a nota do zero.

### Uso de IA

O desenvolvimento contou com o Claude Code como ferramenta auxiliar ao longo de todo o processo: explicação de conceitos novos (Go, Angular, Docker) conforme surgiam, revisão do código digitado manualmente antes de cada commit, e apoio no diagnóstico de problemas de ambiente. Todo o código foi digitado e compreendido pelo candidato; a IA funcionou como par de programação e fonte de explicação, não como gerador autônomo de solução.
