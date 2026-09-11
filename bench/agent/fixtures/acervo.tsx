'use client'

import { useEffect, useState } from 'react'
import { useSearchParams } from 'next/navigation'

type Status = 'pending' | 'processed' | 'failed'

type Document = {
  id: string
  filename: string
  status: Status
  pages: number
  owner?: string
}

type PagedDocument = {
  items: Document[]
  total: number
  page?: number
}

// The token the API wants. It is put here at login and read back on every
// call, which means it is in reach of anything running in the tab.
function token(): string {
  return window.localStorage.getItem('api_token') ?? ''
}

async function listar(q: string): Promise<PagedDocument> {
  const params = new URLSearchParams()
  if (q) params.set('q', q)
  params.set('page_size', '20')
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/documents?${params}`, {
    headers: { Authorization: `Bearer ${token()}` },
  })
  if (!res.ok) throw new Error(`acervo respondeu ${res.status}`)
  return res.json()
}

export default function DocumentosPage() {
  const params = useSearchParams()
  const q = params.get('q') ?? ''

  const [data, setData] = useState<PagedDocument>({ items: [], total: 0 })
  const [loading, setLoading] = useState(true)
  const [erro, setErro] = useState('')

  useEffect(() => {
    let alive = true
    setLoading(true)
    listar(q)
      .then((d) => {
        if (alive) {
          setData(d)
          setErro('')
        }
      })
      .catch((e) => alive && setErro(String(e)))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [q])

  if (loading) return <p className="carregando">Carregando…</p>
  if (erro) return <p className="erro">{erro}</p>

  return (
    <div className="cartao">
      <h1>Documentos</h1>

      <form className="filtro" method="get">
        <input name="q" defaultValue={q} placeholder="Buscar" />
        <button type="submit" className="botao">
          Buscar
        </button>
      </form>

      {data.items.length === 0 ? (
        <p>Nenhum documento corresponde a esta busca.</p>
      ) : (
        <table className="ui-table">
          <thead>
            <tr>
              <th>Documento</th>
              <th>Status</th>
              <th className="ui-num">Páginas</th>
            </tr>
          </thead>
          <tbody>
            {data.items.map((d) => (
              <tr key={d.id}>
                <td>{d.filename}</td>
                <td>
                  <span className="ui-badge">{d.status}</span>
                </td>
                <td className="ui-num">{d.pages}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <p className="total">{data.total} documentos no acervo</p>
    </div>
  )
}
