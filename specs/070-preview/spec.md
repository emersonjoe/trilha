# Spec 070 — Mostrar o arquivo no lugar

- **Issue**: #102 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `070-preview`
- **Versão**: 0.52.0

## Por quê

A tela de documento ao lado dos metadados é a que todo app de gestão tem. No Trilha o
`c.Inline` já servia o PDF do jeito certo, mas quem escrevia `h.Iframe(h.Src(...))` ficava com
um quadro em branco. A issue chamou isso de "a armadilha mais barata de desarmar do framework",
e estava certa — só que sobre o lado errado.

## O achado

A documentação do repositório dizia, em quatro lugares, que o conserto era acrescentar
`frame-src 'self'` no `Security.CSPExtra` da página que enquadra. **Não era, e nunca foi.**

A política padrão já tem `default-src 'self'`, que cobre `frame-src`: a página sempre pôde
enquadrar a própria origem. Quem recusa é a **resposta enquadrada** — toda resposta sai com
`X-Frame-Options: DENY` e `frame-ancestors 'none'`. Medido no navegador, e o console diz com
todas as letras:

```
Framing 'http://localhost/…' violates the following Content Security Policy
directive: "frame-ancestors 'none'". The request has been blocked.
```

Ou seja: quem seguisse a receita da documentação mudava a política da página, continuava com o
quadro em branco, e agora com uma pista a menos, porque a mudança que fez não tinha relação com
o erro que via.

## O que muda

**O `c.Inline` passa a dizer que aquela resposta pode ser enquadrada por uma página da mesma
origem** — `X-Frame-Options: SAMEORIGIN` e `frame-ancestors 'self'`, naquela resposta só. É a
correção de um bug, não uma folga nova: um documento que ninguém pode enquadrar não aparece no
lugar, que é literalmente para o que o `Inline` existe. O `Attachment` continua com `DENY`, e o
app que escreveu o próprio `Security.CSP` ou marcou `Delegated` fica intocado — política escrita
à mão é uma decisão, e editar a decisão de alguém em silêncio é pior que um quadro em branco.

**`ui.Preview`** desenha: barra com título, baixar e abrir em nova aba; quadro para o que o
navegador mostra; `<img>` para imagem, com o clique abrindo o tamanho real (é o zoom que a issue
pediu, sem uma linha de script); e cartão "não dá para pré-visualizar" com o botão de baixar
para o que o `Inline` recusa — em vez de um quadro vazio que não explica nada.

**O sandbox é por tipo, e isso foi medido.** O visualizador de PDF do navegador se recusa a
rodar dentro de um frame com `sandbox`: a requisição volta bloqueada e o quadro fica em branco,
sem nada no console. Os dois flags que o trariam de volta — `allow-scripts` com
`allow-same-origin` — são exatamente o par que deixa um documento de mesma origem tirar o
próprio sandbox, então o atributo seria um rótulo e não uma cerca. PDF vai sem sandbox, e o
resto (texto, imagem) mantém `allow-same-origin`, que foi conferido renderizando.

**`trilha.CanInline`** vira público: o exemplo do blog tinha uma segunda cópia da lista do
`Inline` escrita à mão, e segunda cópia é a que fica para trás.

**O `trilha audit`** avisa sobre `h.Iframe` escrito à mão — com o conselho certo, que é sobre a
resposta enquadrada.

## Desvios da issue

- **Nada é marcado no `Ctx` para a CSP da página.** A issue propunha que o `Preview` marcasse
  `frame-src 'self'` na resposta, "a mesma mecânica do nonce". A medição mostrou que a página
  nunca foi o problema: sob a política padrão ela já podia enquadrar a própria origem. Marcar
  seria acrescentar mecanismo para consertar o que não estava quebrado, e deixar de fora o que
  estava.
- **Sem "página N de M" no PDF.** A issue admitia "só com o que o navegador dá", e o navegador
  não dá: o visualizador embutido não expõe a contagem de páginas para a página que o enquadra.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhuma dependência; dois cabeçalhos e um componente. |
| III — rota no exemplo | `/anexos/{nome}/ver` mostra o anexo; a lista deixou de repetir a lista do `Inline` e usa o `CanInline`. |
| IV — superfície pequena | Uma função, um struct de opções com cinco campos, e um predicado que já existia por dentro. |
| V — inglês no código, pt-BR junto | Referência, receita de uploads e modelo de ameaças nas duas línguas — e a tabela pt do modelo de ameaças, que estava com três linhas a menos que a inglesa, foi emparelhada. |
| VI — teste primeiro | Render (PDF sem sandbox, texto com, imagem em `<img>`, o cartão do tipo recusado), e2e conferindo os cabeçalhos das duas respostas (inline afrouxa, attachment não), teste do aviso do audit, e o navegador para o que teste nenhum prova. |
| VII — segurança por padrão | A folga é de uma resposta, de um tipo de envio e de uma origem; o modelo de ameaças ganhou a linha que a descreve. |

## Tarefas

1. `security.go` + `send.go`: a resposta do `Inline` permite ser enquadrada pela mesma origem. ✅
2. `ui/preview.go` + CSS: `Preview`, `PreviewOpts`, o fallback e o sandbox por tipo. ✅
3. `trilha.CanInline` público; o exemplo do blog deixa de repetir a lista. ✅
4. Aviso do `trilha audit` para `h.Iframe` à mão, nas duas línguas. ✅
5. Correção dos quatro lugares que ensinavam o conserto errado. ✅
6. Receita de uploads "mostrando", modelo de ameaças, referência do `ui`. ✅
