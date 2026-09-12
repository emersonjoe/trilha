export function salvarSessao(valor: string | null) {
  sessionStorage.setItem("sessao", valor ?? "")
}
