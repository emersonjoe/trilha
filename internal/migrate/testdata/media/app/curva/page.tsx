import type { SpeakingFeedback } from "../lib/speaking"

export default function Curva({ f }: { f: SpeakingFeedback }) {
  return <p>{f.ok ? "ok" : "não"}</p>
}
