"use client"

import { useVoiceRecorder } from "../hooks/use-voice-recorder"

export default function Licao() {
  const { start, stop, gravando } = useVoiceRecorder()
  return (
    <button onClick={gravando ? stop : start}>{gravando ? "Parar" : "Gravar"}</button>
  )
}
