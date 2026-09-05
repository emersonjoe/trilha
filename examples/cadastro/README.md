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
- **Responsividade**: `ui-grid` empilha os campos em telas estreitas.

```bash
cd examples/cadastro && trilha dev
```

Teste: `go test ./examples/cadastro/`.
