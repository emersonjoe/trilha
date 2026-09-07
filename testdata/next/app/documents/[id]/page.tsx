"use client"

import { useState, useEffect, useRef } from "react"
import { apiGet, apiPost } from "@/lib/api"

export default function Document({ params }: { params: { id: string } }) {
  const [doc, setDoc] = useState(null)
  const [status, setStatus] = useState("pending")
  const timer = useRef<any>(null)
  useEffect(() => {
    apiGet(`/api/documents/${params.id}`).then(setDoc)
    timer.current = setInterval(() => {
      apiGet(`/api/documents/${params.id}/status`).then((s: any) => setStatus(s.state))
    }, 5000)
    return () => clearInterval(timer.current)
  }, [params.id])
  return (
    <article>
      <button onClick={() => apiPost(`/api/documents/${params.id}/reprocess`, {})}>Reprocess</button>
      <p>{status}</p>
    </article>
  )
}
