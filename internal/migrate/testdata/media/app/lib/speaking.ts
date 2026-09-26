/**
 * O áudio nunca sai do aparelho — o que chega aqui é só a transcrição do
 * SpeechRecognition e o tempo da gravação. Puro/testável.
 */
export type SpeakingFeedback = { ok: boolean }

// Nada de getUserMedia( nem new MediaRecorder( aqui: quem grava é o hook.
export function avaliar(texto: string): SpeakingFeedback {
  const url = "https://example.com/avaliar" // uma URL não é comentário
  return { ok: texto.length > 0 && url !== "" }
}
