# Plano — Spec 027

**Branch**: `027-validacao` · **Spec**: [spec.md](./spec.md) · uma rodada de `make test` por bloco.

## Os fatos que decidem o desenho

1. **O `Bind` já anda a struct inteira** (`bindStruct`, com prefixo de struct aninhada). A
   validação anda a **mesma** árvore, com o mesmo cálculo de nome — então ela é uma segunda
   passada sobre a mesma recursão, não uma cópia dela. Se as duas divergirem, a mensagem cai
   num `name` que o formulário não tem, e o usuário vê um erro sem campo.
2. **A segunda passada precisa ser depois da primeira inteira.** `eqfield` compara com outro
   campo, e a ordem dos campos na struct não pode decidir o resultado. Então: converter tudo,
   coletar (nome → valor), depois aplicar as regras.
3. **A regra recebe o valor convertido, não a string.** `min=18` num `int` é comparação de
   número; num texto é contagem de runas. Isso mora num `Field` com `Value any` e `Text
   string`, e o `switch` fica dentro da regra — é o único lugar do desenho onde o `reflect`
   vaza para fora, e por isso o `Field` esconde o `reflect.Value`.
4. **Campo que falhou na conversão não pode ser validado.** O `bindStruct` já sabe disso;
   basta ele marcar o nome, e a passada de regras pular quem está marcado.
5. **`Validator` é procurado no tipo do campo e no ponteiro para ele.** `func (c *CPF)
   Validate() error` e `func (c CPF) Validate() error` são ambos comuns; o `reflect` precisa
   tentar o endereço quando o campo é endereçável.
6. **Regra desconhecida é pânico, não erro de usuário.** Uma tag errada é um bug do
   desenvolvedor, e um 422 silencioso ("passou porque a regra não existe") é a pior saída
   possível: o formulário aceitaria qualquer coisa em produção.
7. **O mapa de regras é global e escrito no `Setup`.** Ler concorrente é o caso comum; a
   escrita acontece antes do servidor subir. Um `sync.RWMutex` (barato) evita a corrida do
   app que registra regra dentro de um handler.

## Fases

1. **Motor** — `validate.go`: `Field`, `Validator`, `AddRule`, `ValidationMessages`,
   `UseValidationPTBR`, as sete regras e a passada de validação. Teste primeiro em
   `validate_test.go`.
2. **Ligação com o `Bind`** — `bind.go` marca o campo que não converteu, coleta os campos e
   chama a validação; `BindJSON` passa pelo mesmo caminho.
3. **Exemplo `blog`** — o formulário de post usa `Bind` com tags e responde 422 com o
   formulário de volta, no lugar do redirecionamento com mensagem na query string.
4. **Exemplo `orcamento`** — tags no `plano.Lancamento`, `Validar` reduzido à regra de
   negócio, `UseValidationPTBR()` no `Setup`, e um tipo com `Validate()` para o valor em
   dinheiro.
5. **Documentação e fechamento** — capítulo de formulários nas duas locales, referência do
   `Bind`/regras, CHANGELOG, `version`, ROADMAP, release.

## Riscos

- **Nome do campo divergente.** Mitigado por reusar a recursão do `bindStruct` (uma função,
  duas saídas) e por um teste com struct aninhada com prefixo.
- **Mensagem em duas línguas na mesma tela.** `UseValidationPTBR` troca o mapa inteiro,
  inclusive `BindInvalid`; o teste do `examples/orcamento` confere que nenhuma mensagem em
  inglês sobra.
- **Regra nova mudando comportamento de quem não pediu.** Struct sem `validate` não passa por
  regra nenhuma; o teste de `Bind` existente continua valendo sem alteração.
- **`required` no número.** É a decisão mais fácil de errar na leitura; vai documentada com o
  atalho (`*int`) na mesma frase, e com teste dos dois lados.
