"use client"

import * as portalApi from "@/lib/portalApi"

export default function Arquivos({ arquivo }: { arquivo: File }) {
  return <button onClick={() => portalApi.portalUpload(arquivo)}>Enviar</button>
}
