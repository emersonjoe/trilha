import { db } from "@/lib/db"

export async function salvarPerfil(dados: FormData) {
  "use server"
  await db.baralho.create({ data: { nome: dados.get("nome") } })
}

export function formatarNome(nome: string) {
  return nome.trim()
}

export default function Perfil() {
  return (
    <form action={salvarPerfil}>
      <input name="nome" />
    </form>
  )
}
