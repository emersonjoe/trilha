"use client"

import { useRef } from "react"

export default function Ouvir() {
  const som = useRef<HTMLAudioElement | null>(null)
  const tocar = () => {
    som.current = new Audio("/audio/licao-1.mp3")
    som.current.play()
  }
  return <button onClick={tocar}>Tocar</button>
}
