# Spec 072 — A tela da trilha

- **Issue**: #128 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `072-audittable`
- **Versão**: 0.54.0

## Por quê

A #104 (0.49.0) deu ao framework o `c.Audit` e deixou a tela de fora com o motivo escrito:
dependia do `c.CSV`, e sem o botão de exportar seria um `DataTable` de cinco colunas. O `c.CSV`
saiu na 0.50.0; a dependência caiu.

## O que muda

`ui.AuditTable(c, registros, opts)`. O contrato está na referência de observabilidade, nas duas
línguas. O que vale registrar:

**É um `DataTable` por baixo, e é esse o ponto.** Filtro, ordem, paginação e troca de fragmento
são os mesmos de qualquer listagem do app. Uma trilha que se comportasse diferente do resto
seria uma segunda coisa para aprender — e teria a própria cópia de quatro mecanismos que já
existem.

**Ler a trilha continua sendo da aplicação.** O `Config.Audit` é interface de escrita de um
método e não ganhou um `Read`: o framework não tem banco, e a consulta desta tela — período,
ator, a tabela que aquele app escolheu — não é algo que ele pudesse escrever. A do exemplo tem
trinta linhas sobre uma fatia.

**`Fields` é detalhe, não coluna.** Cada ação carrega as suas chaves; uma coluna por chave é uma
tabela que ganha coluna toda vez que alguém audita algo novo. As chaves saem ordenadas, porque
detalhe que embaralha entre dois carregamentos da mesma tela é detalhe em que ninguém confia.

**A exportação leva o recorte da tela.** Exportar ignorando o filtro na frente da pessoa é
exportar a coisa errada, e ela só descobre na planilha.

**O anônimo aparece.** Mesma decisão da #104, agora do lado do desenho: trilha que some com a
ação anônima tem um buraco exatamente onde alguém vai procurar. O selo de `via` só aparece para
o que não é uma pessoa no teclado — uma chave, o sistema, ninguém.

## O que o exemplo mostrou

A exportação nasceu em `app/auditoria_csv/`, fora da pasta guardada. O gerador denunciou duas
coisas de uma vez: a URL saiu `/auditoria_csv` **e a rota não tinha middleware nenhum** — seria o
único endereço entregando a trilha inteira para qualquer um. Passou para
`app/auditoria/csv/`, onde herda o guarda da pasta sem escrever uma linha sobre permissão, e o
teste cobre justamente isso: anônimo e analista tomam 403 no arquivo, não só na tela.

Junto: o `local-login` não declarava `Config.Locale`, então a tela nova falava inglês num app
escrito em português — o mesmo achado da 0.50.0 no blog, no outro exemplo.

## Fora de escopo

- **Filtro por período** — a issue pede; o recorte de datas é da consulta da aplicação, e um
  par de campos aqui só teria valor se o componente também montasse a consulta, que é o que ele
  deliberadamente não faz. O `q` e a ação cobrem o caminho comum; o exemplo mostra onde entra o
  resto.
- **`audit.SQL`** — segue fora, pela mesma razão da 0.49.0.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo; o componente é composição do que já existe. |
| III — rota no exemplo | `/auditoria` e `/auditoria/csv` no `local-login`, guardadas pelo módulo de administração, com e2e. |
| IV — superfície pequena | Uma função e um struct de opções; o resto é o `DataTable`. |
| V — inglês no código, pt-BR junto | Referência de observabilidade e do `ui` nas duas línguas. |
| VI — teste primeiro | Render (quem/ação/alvo, anônimo com selo, detalhe ordenado, exportação com o recorte, estado vazio) e e2e do 403 no arquivo. |
| VII — segurança por padrão | A tela e a exportação atrás do mesmo guarda, e o aviso escrito na referência; quem exporta a trilha inteira também é auditado. |

## Tarefas

1. `ui/audit.go` + CSS do detalhe. ✅
2. `/auditoria` e `/auditoria/csv` no `local-login`, dentro da pasta guardada. ✅
3. `Config.Locale` do exemplo. ✅
4. Referência de observabilidade e do `ui`, en e pt. ✅
5. `api/current.txt`, catálogo do `ui`, cópias do kit. ✅
