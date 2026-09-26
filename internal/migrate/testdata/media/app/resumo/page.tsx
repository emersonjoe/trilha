import * as falar from "../lib/speaking"

export default function Resumo() {
  return <p>{falar.avaliar("oi").ok ? "ok" : "não"}</p>
}
