# Spec 148 — o arquivo de `Public` antes da rota curinga, e a especificidade por segmento

- **Issues**: [#203](https://github.com/emersonjoe/trilha/issues/203) e
  [#204](https://github.com/emersonjoe/trilha/issues/204) — as issues são a fonte do escopo;
  aponte para elas, não as reescreva aqui.
- **Branch**: `feat/rotas-public-especificidade`
- **Versão**: 0.127.0

## Por quê

As duas issues são a mesma frase dita de dois jeitos: **o roteamento do kit é o do
`http.ServeMux` e o app que chega pensa nos termos do Next**. Quem porta um app traz duas
árvores de URL prontas — uma de telas, uma de arquivos — e as duas batem no mesmo lugar.

Na #203 o custo já foi pago: um app com `/{lang}` na raiz rodou três sessões sem folha de
estilo, porque `serveStatic` só é chamado do `fallback`, que é a rota `"/"` do mux, e o mux
entrega `/ui.css` para `/{lang}` antes disso. O sintoma é silencioso — a página pedida
responde certo, o 401/303 sai só para o `.css` — e `c.Asset()` não tem como avisar: ele
confere que o arquivo existe, não que a URL chega nele.

Na #204 o custo é um `panic` no boot com dois padrões que o Next resolve sem hesitar. A regra
do Go (especificidade só quando um padrão é especialização-prefixo do outro) é defensável, mas
não é a que o app portado tem escrita nas suas URLs, e nenhuma das duas é negociável: elas já
foram dadas a clientes.

## O que muda

### 1. Um arquivo de `Public`/`Mounts` responde antes de uma rota com curinga

A ordem de despacho de um `GET`/`HEAD` passa a ser:

1. a rota **literal** que casa exatamente com o caminho (nenhum curinga no padrão);
2. o arquivo de `Config.Mounts`/`Config.Public`, se existir;
3. a rota com curinga que casaria com o caminho;
4. o `fallback` de hoje (arquivo estático quando nenhuma rota casa, 405, upstream, 404).

Ou seja: **estático ganha de curinga, rota literal ganha de estático**. `/ui.css` chega no
arquivo mesmo com `/{lang}` na raiz; `app/painel/page.go` continua ganhando de um
`public/painel` que alguém deixou lá, porque quem escreveu a rota literal escreveu o endereço.

O custo por requisição é um `fs.Stat` — e só nos `GET`/`HEAD` de um app que tem estático
configurado **e** cuja rota casada tem curinga. Um app sem `Public` e sem `Mounts`, ou uma
requisição que cai numa rota literal, não paga nada.

`c.Asset()` passa a avisar (uma vez por caminho, como o aviso de arquivo inexistente) quando o
arquivo existe mas o endereço é de uma rota literal — o único caso que sobra em que a URL
devolvida não chega no arquivo. Fora dele, `Asset` só devolve URL que chega.

### 2. Especificidade por segmento para o que o mux recusa

`Register` deixa de repassar todo padrão direto para o `http.ServeMux`. Cada padrão é
oferecido primeiro a um mux de sondagem (o `pathMux`, que já existe); o que ele aceita continua
sendo despachado pelo mux do Go, com tudo que isso traz. O punhado de padrões que ele **recusa**
— os que se cruzam sem que nenhum seja especialização do outro — passa a ser despachado pelo
kit, com a regra do Next:

> Segmento estático ganha de `{param}`, que ganha de `{path...}`, posição por posição, da
> esquerda para a direita. Empate em todas as posições comparáveis: ganha quem tem mais
> segmentos.

Com os dois padrões da #204 registrados, `/o/cards/login` é de `/o/{slug}/login` (o `o`
literal ganha do `{lang}`) e `/en/cards/mazo-1` é de `/{lang}/cards/{deckId}`. Nenhum
`panic`.

A regra é **total e independente da ordem de registro**: dois padrões que casam o mesmo
caminho diferem em alguma posição, e a comparação é sobre o tipo do segmento, não sobre quem
chegou antes. Por isso a ordem em que o gerador emite os `Register` não muda resposta nenhuma.

Onde o mux do Go aceita, ele continua decidindo — e decide igual: entre dois padrões que se
cruzam **sem conflito**, "mais específico" no Go significa que os caminhos de um são um
subconjunto dos do outro, o que só acontece quando ele tem segmento estático onde o outro tem
curinga. A regra nova não contradiz a antiga; ela a estende para o caso em que a antiga
desistia.

O que o despacho do kit precisa reproduzir do mux, e reproduz: `PathValue` de cada curinga
(inclusive o `{path...}`, com as barras), `HEAD` atendido pelo handler de `GET`, 405 com
`Allow` para método que a rota não tem, e o redirecionamento de barra final.

## Fora de escopo

- **Substituir o `http.ServeMux`.** O mux do Go continua sendo o roteador do app inteiro; o
  despacho do kit existe para o conjunto que ele recusa, que num app típico é vazio. Trocar o
  roteador seria pagar em código e em risco por um caso de borda.
- **Aviso no `trilha check`** (a alternativa "no mínimo" da #203): com o arquivo respondendo
  antes da rota curinga, o sombreamento que a issue descreve deixa de existir, e o que sobra
  (rota literal com o mesmo endereço de um arquivo) é deliberado e sai no log do `Asset`. Um
  aviso estático no CLI custaria ler o `public/` do projeto para repetir o que o runtime já
  sabe.
- **Arquivo de `Public` ganhando de rota literal.** Seria trocar um sombreamento silencioso por
  outro, e o endereço escrito à mão é o que o autor do app quis.
- **`%2F` e caminho com barra codificada** no despacho do kit: ele casa sobre `URL.Path`, já
  decodificado, como o resto do runtime.
- **Reordenar os `Register` no gerador.** A regra é aplicada no despacho; o arquivo gerado
  continua em ordem alfabética, que é o que o torna determinístico e legível.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — arquivo é rota | nenhuma convenção nova em `app/`; o que muda é como as rotas já geradas são despachadas |
| II — só biblioteca padrão | `net/http`, `strings`, `path`; nada novo |
| III — geração explícita | o gerador não muda; `examples/blog/trilha_gen.go` é regerado e commitado porque o exemplo ganha rotas |
| IV — API pública pequena e vigiada | nenhum símbolo público novo; `api/current.txt` não muda |
| VI — teste primeiro | `routing_test.go` entra falhando nos dois contratos (o `/ui.css` sob `/{lang}`; os dois padrões da #204 sem `panic`, com `/o/cards/login` na primeira rota) |
| VII — segurança por padrão | o despacho do kit passa pelo mesmo `wrap` (CSRF, middleware, recover, log, métrica) — um handler não é alcançado por fora da cadeia; o estático continua sem listar diretório e sem sair de `Public` |
| Idioma | código e mensagens em inglês; referência de convenções em `/reference/conventions` e `/pt/referencia/convencoes` no mesmo commit |

## Aceitação

- app com `/{lang}` na raiz e `Public` com `ui.css`: `GET /ui.css` responde 200 com
  `Content-Type: text/css`, e `GET /en` continua na rota;
- a rota literal continua ganhando do arquivo de mesmo endereço, e `Asset` avisa nesse caso;
- `Register` com `/o/{slug}/login` e `/{lang}/cards/{deckId}` não entra em `panic`, nas duas
  ordens de registro; `/o/cards/login` cai na primeira e `/en/cards/mazo-1` na segunda, cada
  uma com seus `Param`;
- na rota despachada pelo kit: `POST` sem handler dá 405 com `Allow`, `HEAD` responde como
  `GET`, `/o/x/login/` redireciona para `/o/x/login`, e `c.Pattern()` é o padrão;
- dois `Register` com o mesmo padrão continuam sendo `panic` (é bug do arquivo gerado, não
  conflito);
- `make test` verde, `examples/blog` com as rotas que provam os dois contratos e
  `trilha_gen.go` regerado, `CHANGELOG` e `ROADMAP` fechados na 0.127.0.
