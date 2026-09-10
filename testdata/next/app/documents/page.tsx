"use client"

import { useState, useEffect } from "react"
import { apiGet } from "@/lib/api"
import { DataTable } from "@/components"

export default function Documents() {
  const [rows, setRows] = useState([])
  const [query, setQuery] = useState("")
  useEffect(() => {
    apiGet(`/api/documents?q=${query}`).then(setRows)
  }, [query])
  return <DataTable rows={rows} />
}
