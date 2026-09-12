"use client"

import { formatar } from "@/lib/eventos"

export default function Eventos({ evento }: { evento: MessageEvent }) {
  return <p>{formatar(evento)}</p>
}
