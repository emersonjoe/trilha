import { registrarResposta, criarBaralho } from "@/lib/actions"

export default function Estudo() {
  return (
    <main>
      <form action={registrarResposta}>
        <button type="submit">Acertei</button>
      </form>
      <form action={criarBaralho}>
        <input name="nome" />
      </form>
    </main>
  )
}
