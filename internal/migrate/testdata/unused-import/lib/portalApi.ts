import { salvarSessao } from "./storage"

const BASE = "/api/portal"

async function post(path: string, body: unknown) {
  return fetch(`${BASE}${path}`, { method: "POST", body: JSON.stringify(body) })
}

export async function portalPublico(token: string) {
  const r = await fetch(`${BASE}/convite/${token}`)
  return r.json()
}

export async function entrarNoPortal(token: string, senha: string) {
  return post(`/convite/${token}`, { senha })
}

export async function portalUpload(file: File) {
  const fd = new FormData()
  fd.append("file", file)
  salvarSessao(localStorage.getItem("sessao"))
  return fetch(`${BASE}/arquivos`, { method: "POST", body: fd })
}
