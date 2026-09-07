"use client"

import { useState, useEffect } from "react"
import { apiGet } from "@/lib/api"

export default function Documents() {
  const [rows, setRows] = useState([])
  const [query, setQuery] = useState("")
  useEffect(() => {
    apiGet(`/api/documents?q=${query}`).then(setRows)
  }, [query])
  return (
    <table>
      <tbody>
        {rows.map((r: any) => (
          <tr key={r.id}>
            <td>{r.name}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
