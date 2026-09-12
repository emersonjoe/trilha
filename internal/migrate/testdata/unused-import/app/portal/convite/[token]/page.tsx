"use client"

import { useState } from "react"
import { entrarNoPortal, portalPublico } from "@/lib/portalApi"

export default function Convite({ params }: { params: { token: string } }) {
  const [senha, setSenha] = useState("")
  const [convite, setConvite] = useState(null)
  useState(() => portalPublico(params.token).then(setConvite))
  return (
    <form method="post" onSubmit={() => entrarNoPortal(params.token, senha)}>
      <input type="password" value={senha} onChange={(e) => setSenha(e.target.value)} />
      <button type="submit">Entrar</button>
    </form>
  )
}
