import { registrarResposta, criarBaralho } from "@/lib/estudo-actions"

export default function Cards({ params }: { params: { deckId: string } }) {
  return (
    <main>
      <form action={registrarResposta}>
        <input type="hidden" name="cardId" value={params.deckId} />
        <button type="submit">Acertei</button>
      </form>
      <form action={criarBaralho}>
        <input name="nome" />
      </form>
    </main>
  )
}
