'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { apiGet } from '@/lib/api'

type Documento = {
  id: string
  nome: string
  tipo: string
  bytes: number
  status: 'fila' | 'processando' | 'pronto'
}

type DocsResponse = {
  items: Documento[]
  total: number
}

const TIPOS = ['nota', 'recibo', 'contrato']
const POR_PAGINA = 10

function tamanho(bytes: number): string {
  if (bytes >= 1024) return `${Math.floor(bytes / 1024)} kB`
  return `${bytes} B`
}

export default function DocumentosPage() {
  const router = useRouter()
  const params = useSearchParams()

  const q = params.get('q') ?? ''
  const tipo = params.get('tipo') ?? ''
  const sort = params.get('sort') ?? 'nome'
  const dir = params.get('dir') ?? 'asc'
  const page = Number(params.get('page') ?? '1')

  const [data, setData] = useState<DocsResponse>({ items: [], total: 0 })
  const [loading, setLoading] = useState(true)

  // The listing reloads every five seconds because documents are processed in
  // the background and the status changes underneath the user.
  useEffect(() => {
    let alive = true
    const carregar = async () => {
      const qs = new URLSearchParams({
        q, tipo, sort, dir,
        offset: String((page - 1) * POR_PAGINA),
        limit: String(POR_PAGINA),
      })
      const res = await apiGet<DocsResponse>(`/api/documents?${qs}`)
      if (alive) {
        setData(res)
        setLoading(false)
      }
    }
    carregar()
    const t = setInterval(carregar, 5000)
    return () => { alive = false; clearInterval(t) }
  }, [q, tipo, sort, dir, page])

  const irPara = (next: Record<string, string>) => {
    const qs = new URLSearchParams({ q, tipo, sort, dir, page: String(page), ...next })
    router.push(`/documentos?${qs}`)
  }

  const ordenarPor = (coluna: string) => {
    const novaDir = sort === coluna && dir === 'asc' ? 'desc' : 'asc'
    irPara({ sort: coluna, dir: novaDir, page: '1' })
  }

  const paginas = Math.ceil(data.total / POR_PAGINA)

  return (
    <div>
      <h1>Documentos</h1>

      <form onSubmit={(e) => { e.preventDefault(); irPara({ page: '1' }) }}>
        <input
          name="q"
          defaultValue={q}
          placeholder="Buscar"
          onBlur={(e) => irPara({ q: e.target.value, page: '1' })}
        />
        <select value={tipo} onChange={(e) => irPara({ tipo: e.target.value, page: '1' })}>
          <option value="">Todos os tipos</option>
          {TIPOS.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
      </form>

      {loading && <p>Carregando…</p>}

      <table>
        <thead>
          <tr>
            <th aria-sort={sort === 'nome' ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
              <button onClick={() => ordenarPor('nome')}>Documento</button>
            </th>
            <th>Tipo</th>
            <th aria-sort={sort === 'tamanho' ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
              <button onClick={() => ordenarPor('tamanho')}>Tamanho</button>
            </th>
            <th aria-sort={sort === 'status' ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
              <button onClick={() => ordenarPor('status')}>Status</button>
            </th>
          </tr>
        </thead>
        <tbody>
          {data.items.map((d) => (
            <tr key={d.id}>
              <td>{d.nome}</td>
              <td><span className="badge">{d.tipo}</span></td>
              <td>{tamanho(d.bytes)}</td>
              <td><span className="badge">{d.status}</span></td>
            </tr>
          ))}
        </tbody>
      </table>

      {data.items.length === 0 && !loading && <p>Nenhum documento encontrado.</p>}

      <nav>
        {page > 1 && <a href={`/documentos?q=${q}&tipo=${tipo}&sort=${sort}&dir=${dir}&page=${page - 1}`}>Anterior</a>}
        <span>Página {page} de {paginas}</span>
        {page < paginas && <a href={`/documentos?q=${q}&tipo=${tipo}&sort=${sort}&dir=${dir}&page=${page + 1}`}>Próxima</a>}
      </nav>
    </div>
  )
}
