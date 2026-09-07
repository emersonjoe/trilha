"use client"

import { useState, useMemo } from "react"
import { apiGet } from "@/lib/api"

export default function Dashboard() {
  const [range, setRange] = useState("30d")
  const data = useMemo(() => apiGet(`/api/metrics?range=${range}`), [range])
  return (
    <svg viewBox="0 0 600 200" onPointerDown={() => setRange("7d")}>
      <polyline points="0,0 600,200" />
    </svg>
  )
}
