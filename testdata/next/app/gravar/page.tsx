"use client"

import { useRef, useState } from "react"

export default function Gravar() {
  const rec = useRef<MediaRecorder | null>(null)
  const [gravando, setGravando] = useState(false)
  const start = async () => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    rec.current = new MediaRecorder(stream)
    rec.current.start()
    setGravando(true)
  }
  return <button onClick={start}>{gravando ? "Gravando" : "Gravar"}</button>
}
