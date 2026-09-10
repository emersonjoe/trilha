"use client"

import { useEffect, useRef, useState } from "react"

export default function Chat() {
  const [messages, setMessages] = useState<string[]>([])
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const t = setInterval(() => setMessages((m) => m), 5000)
    return () => clearInterval(t)
  }, [])
  return (
    <aside ref={box}>
      <svg viewBox="0 0 16 16"><circle cx="8" cy="8" r="7" /></svg>
      <ul>{messages.map((m) => <li key={m}>{m}</li>)}</ul>
    </aside>
  )
}
