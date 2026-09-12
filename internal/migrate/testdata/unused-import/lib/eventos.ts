// The channel opens when the module is imported, not when somebody calls the
// function below: it is the work of every screen that imports anything here.
const canal = new EventSource("/api/eventos")

export function assinar(fn: (e: MessageEvent) => void) {
  canal.addEventListener("message", fn)
}

export function formatar(e: MessageEvent) {
  return String(e.data)
}
