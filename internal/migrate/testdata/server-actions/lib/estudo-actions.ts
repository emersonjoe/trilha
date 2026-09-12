"use server"

import { db } from "./db"

export async function registrarResposta(cardId: string, acertou: boolean) {
  await db.resposta.create({ data: { cardId, acertou } })
}

export async function criarBaralho(nome: string) {
  await db.baralho.create({ data: { nome } })
}

export async function apagarBaralho(id: string) {
  await db.baralho.delete({ where: { id } })
}
