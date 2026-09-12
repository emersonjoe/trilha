import { useRef, useState } from "react"

export function useVoiceRecorder() {
  const rec = useRef<MediaRecorder | null>(null)
  const [gravando, setGravando] = useState(false)
  const start = async () => {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    rec.current = new MediaRecorder(stream)
    rec.current.start()
    setGravando(true)
  }
  const stop = () => {
    rec.current?.stop()
    setGravando(false)
  }
  return { start, stop, gravando }
}
