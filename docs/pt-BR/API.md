# API pública e política de depreciação

> 🇧🇷 Português · [🇺🇸 English](../../API.md)

Este documento traça a linha entre o que o framework promete e o que ele apenas expõe hoje. O
que está do lado prometido muda segundo a política abaixo; o que está do outro lado pode mudar
em qualquer versão, sem aviso.

## O que está coberto

Os símbolos exportados destes pacotes, com o comportamento que a documentação deles descreve:

| Pacote | O que é |
|---|---|
| `github.com/emersonjoe/trilha` | o runtime: `App`, `Config`, `Ctx`, `Route`, erros, cliente de teste |
| `.../h` | a DSL de HTML: `Node`, elementos, atributos, `Render` |
| `.../ui` | o kit de componentes construído sobre o `h` |
| `.../tmpl` | `html/template` dentro de uma página |
| `.../ai` | o cliente de modelo e o laço de agente |
| `.../ai/mcp` | o servidor e o cliente MCP |
| `.../auth` | OpenID Connect: `Provider`, `Config`, `Store`, os handlers prontos |
| `.../cache` | a interface de cache e a implementação em memória |

A lista completa de símbolos está em [`api/current.txt`](../../api/current.txt), uma linha por
símbolo, gerada a partir do código.

## O que não está coberto

- Tudo em `internal/`. O compilador do Go já garante isso; está escrito aqui para ninguém
  procurar um jeito de contornar.
- O binário `cmd/trilha`: os comandos e as flags são documentados e estáveis na prática, mas a
  saída exata — texto, ordem das colunas, códigos de saída além de `0`/`1` — não é contrato.
- O conteúdo do `trilha_gen.go` gerado. Ele é regravado pela ferramenta que o escreveu.
- O HTML que o `ui` produz: classes, ordem dos elementos e estrutura mudam conforme o kit
  evolui. O que se promete é a assinatura da função e para que o componente serve, não a
  marcação dele.
- Os pacotes de `site/` e `examples/`. São aplicações que moram no repositório, não biblioteca.
- Qualquer coisa que a documentação chame de experimental.

## O que conta como quebra

- Remover ou renomear símbolo exportado.
- Mudar assinatura de função ou método, inclusive o tipo de um parâmetro ou resultado.
- Tirar campo de struct exportada, ou mudar o tipo dele.
- Acrescentar método a uma interface que se espera que quem chama implemente.
- Apertar o que uma função aceita, ou afrouxar o que ela garante, mesmo sem mexer na
  assinatura — um campo de `Config` que passa a ser obrigatório, um erro onde antes vinha o
  valor zero.

**Não** é quebra: acrescentar função, método, campo de struct ou constante. Por causa disso,
preencha struct de configuração por nome de campo (`trilha.Config{Env: trilha.Prod}`), nunca
posicionalmente — um campo novo quebraria a forma posicional, e não é para quebrar.

## Política de depreciação

Antes de um símbolo coberto sumir:

1. O doc comment dele ganha um parágrafo `Deprecated:` dizendo o que usar no lugar.
2. O `CHANGELOG.md` o lista em `Deprecated`, na versão em que a marca aparece.
3. Ele continua funcionando por **pelo menos uma versão menor** ao lado do substituto.

```go
// Sign returns the signed value of v.
//
// Deprecated: use Signer.Sign, which takes the expiry explicitly.
func Sign(v string) string { … }
```

A única exceção é correção de segurança que não dá para fazer de forma compatível. Quando
acontecer, o CHANGELOG diz isso com essas palavras, e as notas da versão explicam a migração.

Enquanto o projeto está em `0.x`, uma quebra pode entrar em versão menor — `0.24 → 0.25` —
como o `GOVERNANCE.md` já diz. A política acima continua valendo: o ciclo de depreciação é o
que transforma "pode quebrar" em "terá sido anunciado".

Depois da `1.0`, quebra só em versão maior, e símbolo depreciado sobrevive até a maior
seguinte.

## Como a linha é vigiada

O `api/current.txt` é gerado do código e versionado. Um teste o compara a cada rodada:

```bash
make test   # falha se a superfície exportada diferir do arquivo
make api    # regrava o arquivo, depois de uma mudança intencional
```

A falha lista o que entrou e o que saiu. Quem revisa a mudança vê a linha removida no diff — e
esse é o ponto todo, já que tirar um campo de uma struct é, de outro modo, uma linha que
compila sem reclamação deste lado da cerca.

Símbolo com `Deprecated:` aparece marcado no arquivo, então a revisão vê também quando algo
entra em depreciação, e quando algo sai sem ter passado por ela.

**O que o teste não pega**: comportamento. Uma função que mantém a assinatura e passa a
devolver outra coisa atravessa a verificação. Para isso servem o CHANGELOG e os testes de cada
funcionalidade; o arquivo de superfície guarda a compilação, não a semântica.
