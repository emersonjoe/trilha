"use client"

export default function Ditado() {
  const ouvir = () => {
    const R = window.SpeechRecognition || window.webkitSpeechRecognition
    const r = new R()
    r.lang = "pt-BR"
    r.start()
  }
  return <button onClick={ouvir}>Falar</button>
}
