# Spec 075 — Cifre isto para guardar

- **Issue**: #108 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `075-seal`
- **Versão**: 0.57.0

## Por quê

O framework tinha segredo, assinador e cookies assinados. Não tinha "cifre isto para guardar" —
e o que se escreve no lugar é um token em claro, depois um AES copiado da internet com IV fixo,
depois a chave inteira voltando num `GET` e aparecendo no DevTools.

## O que muda

`trilha.Seal`/`Open`, o tipo `trilha.Secret`, o campo de senha no `Schema` e o
`ui.SecretField`. O contrato está na referência de segurança e no modelo de ameaças, nas duas
línguas. O que vale registrar:

**A cifra.** AES-256-GCM, nonce aleatório por selo, chave derivada do segredo da app por
HKDF-SHA256 com info fixa — a chave que cifra nunca é a que assina. O primeiro byte é a versão
do formato e viaja como dado adicional, então trocá-lo invalida a tag em vez de virar outro
formato. `Open` tenta o segredo atual e depois o `PreviousSecret`, que é o que faz a rotação
existir.

**Um erro só para "não é meu" e "não abre".** Distinguir os dois conta a quem está tentando qual
dos dois acertou.

**`Value()` é do driver, e `Reveal()` é o leitor.** A issue pedia o contrário e não dá: um
`Secret` é string por baixo, e o `database/sql` converte sozinho qualquer valor de kind string.
Sem `Value()` sendo o `driver.Valuer`, passar um `Secret` para uma query grava o texto em claro,
em silêncio — exatamente o acidente que o tipo existe para impedir. O nome vai para o método que
evita o vazamento; o leitor fica com um nome que diz o que faz.

**Máscara voltando é "não mudou".** Formulário não tem como mandar "não mexi nisto". `Bind`
trata vazio **e máscara** como não mudou, e o `Settings` desenha um `Secret` como campo de senha
sempre vazio.

## O defeito que a composição com a 0.56.0 revelou

Ao pôr a chave do provedor na seção de configuração do exemplo, o `SchemaOf` desenhou o `Secret`
como campo de texto — porque é string por baixo — e eu ainda somei um `ui.SecretField` ao
formulário: **dois inputs com o mesmo nome na mesma tela**. O conserto é o tipo `password` no
`Schema`, que o `SchemaOf` reconhece antes de olhar o kind, com valor sempre vazio; o
`ui.SecretField` continua existindo para formulário escrito à mão. A dica de "qual segredo está
guardado" mora no `Settings.Schema()`, que é o único lugar que conhece o valor — o `SchemaOf` é
de tipo.

## Fora de escopo, com o motivo

- **Gerenciador de chaves (KMS, Vault).** A chave vem do segredo da app, e está escrito nos três
  lugares onde alguém vai ler: isto protege dump e backup, não o operador. Fingir o contrário
  seria pior que não cifrar.
- **Re-selagem automática na rotação.** Quem sabe onde os valores estão guardados é a aplicação;
  o framework não tem banco para varrer. O `trilha audit` avisa antes de a rotação quebrar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`, `database/sql`. HKDF escrito com o `hmac` da stdlib, sem `x/crypto`. |
| III — rota no exemplo | A chave do provedor no `assistente`, dentro da seção de configuração da 0.56.0. |
| IV — superfície pequena | Duas funções, um tipo, um componente. |
| V — inglês no código, pt-BR junto | Referência de segurança e modelo de ameaças nas duas línguas. |
| VI — teste primeiro | Abre o que fechou, não repete selo, recusa byte mexido e versão trocada, rotação com e sem `PreviousSecret`, sem segredo não devolve claro; JSON, log e `%v` mascarados; a coluna recebe o selo; `Bind` vazio e mascarado preservam. |
| VII — segurança por padrão | O aviso do `audit` sobre rotação, e a linha do modelo de ameaças dizendo contra o que isto protege e contra o que não. |

## Tarefas

1. `seal.go`: `Seal`, `Open`, `Secret`, `SecretFrom`, máscara, `Valuer`/`Scanner`. ✅
2. `bind.go`: vazio e máscara preservam o guardado. ✅
3. `Schema` tipo `password`, `SchemaOf` reconhecendo `Secret`, `Settings.Schema()` com a dica. ✅
4. `ui/secret.go` e o render do `password` no `SchemaForm`. ✅
5. Aviso de rotação no `trilha audit`, nas duas línguas. ✅
6. Referência de segurança, modelo de ameaças, exemplo. ✅
