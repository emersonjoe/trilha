"use client"

import { useState } from "react"
import Chat from "./Chat"

export default function Shell({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  return (
    <div>
      <nav><button onClick={() => setOpen(!open)}>Menu</button></nav>
      <main>{children}</main>
      <Chat />
    </div>
  )
}
