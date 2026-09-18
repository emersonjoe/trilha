package coleta

// Offline põe esta tela na lista que o service worker guarda: ela abre sem
// rede, e o formulário dela espera no outbox até a rede voltar. A convenção é
// lida como `var Kind` e `var CORS` — qualquer arquivo do pacote a declara.
var Offline = true
