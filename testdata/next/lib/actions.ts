"use server"

export async function registrarResposta(cardId: string, acertou: boolean) {
  await db.resposta.create({ data: { cardId, acertou } })
}

export async function criarBaralho(nome: string) {
  await db.baralho.create({ data: { nome } })
}

// No screen imports this one, and the FormData in it was enough to call the
// screen that imports the other two an upload island.
export async function enviarArquivo(arquivo: File) {
  const fd = new FormData()
  fd.append("arquivo", arquivo)
  await fetch("/api/arquivos", { method: "POST", body: fd })
}
