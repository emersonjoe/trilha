# Exemplo: cadastro (dificuldade média)

Formulário com regras de negócio, sem framework de JavaScript.

O que ensina:

- **Campos condicionais** com `ui.ShowWhen`: tipo PF/PJ, endereço de cobrança, frequência de
  novidades. Escondido = desabilitado, então não vai no `POST`.
- **Validação no servidor** com `c.Bind(&struct)` + `trilha.FieldErrors`; a página volta com
  `c.Render(422, …)`, mensagens no campo (`ui.Errors`, `ui.InvalidIf`) e valores preservados
  (`h.Value`, `ui.Checked`, `ui.SelectOptions`).
- **Seleção dependente** (UF → cidade) por uma rota de API e 20 linhas de `app.js`.
- **Feedback que some**: `ui.Toast("success", …, 4000)` após o redirect (`/?ok=1`).
- **Lista de sub-registros**: os dependentes vão e voltam como `dependentes[0].nome`, que é o
  nome do input, a chave da mensagem e o que o `Bind` lê de novo; a linha em branco do fim é a
  próxima, sem JavaScript.
- **Formulário definido por dado** em `/ficha`: o esquema chega como JSON, `trilha.BindSchema`
  lê e valida, `ui.SchemaForm` desenha.
- **Responsividade**: `ui-grid` empilha os campos em telas estreitas.

```bash
cd examples/cadastro && trilha dev
```

Teste: `go test ./examples/cadastro/`.
