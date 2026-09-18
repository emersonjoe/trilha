package recipes

// whatsappRecipe is the other direction of the webhook: somebody else's
// events arriving here.
//
// `webhook.Verify` checks Trilha's own scheme. An application that receives
// from Meta goes back to writing hmac.Equal by hand, with the mistakes the
// module exists to prevent — and then writes, again, the normalisation of a
// payload that is an envelope inside an envelope, and the dedup that keeps a
// provider's retry from becoming two messages.
//
// The recipe is the second half of that: webhook.VerifyHMAC does the
// signature, and what lands in the project is the normalisation, the contract
// (Message, Status, Receber), the dedup, and the two calls that answer.
func whatsappRecipe() Recipe {
	return Recipe{
		Name: "channel-whatsapp",
		Summary: map[string]string{
			"en": "receiving from WhatsApp Cloud API: signature, normalised message, dedup, and the two ways to answer",
			"pt": "recebendo da Cloud API do WhatsApp: assinatura, mensagem normalizada, dedup e os dois jeitos de responder",
		},
		Doc: "/cookbook/webhooks",
		// The credentials are two sealed connections, so the screen that
		// edits them has to exist first. Without it the app secret would end
		// up in an environment variable, which is the thing this recipe is
		// written to avoid.
		Needs: []Need{{Recipe: "connections", File: "internal/conexoes/conexoes.go"}},
		Files: []File{
			{Rel: "internal/whatsapp/whatsapp.go", Go: true, Body: waDecl},
			{Rel: "internal/whatsapp/whatsapp_test.go", Go: true, Body: waDeclTest},
			// The address is fixed on purpose: it is typed into somebody
			// else's dashboard, so it does not move with --at the way a
			// screen does.
			{Rel: "app/webhooks/whatsapp/route.go", Go: true, Body: waRoute},
			{Rel: "app/webhooks/whatsapp/kind.go", Go: true, Body: waKind},
			{Rel: "whatsapp_test.go", Go: true, Body: waTest},
		},
		Next: map[string]string{
			"en": "Open {{.URL}}conexoes and create two connections of kind API: one named `whatsapp` " +
				"(auth bearer, secret the access token, URL https://graph.facebook.com/v21.0/<phone_number_id>) " +
				"and one named `whatsapp-webhook` (auth header, header X-Hub-Signature-256, secret the app secret). " +
				"Then point Meta at https://<your host>/webhooks/whatsapp with the verify token " +
				"`whatsapp.VerifyToken(<the app secret>)` — it is derived from the secret, so there is no third " +
				"thing to store. Fill in `whatsapp.Receber` in app/setup.go: it runs inside the request, so it " +
				"enqueues and returns. And read the 24-hour window in the doc comment of SendText before you " +
				"promise anybody an answer tomorrow.",
			"pt": "Abra {{.URL}}conexoes e crie duas conexões do tipo API: uma chamada `whatsapp` " +
				"(auth bearer, segredo o token de acesso, URL https://graph.facebook.com/v21.0/<phone_number_id>) " +
				"e uma chamada `whatsapp-webhook` (auth header, cabeçalho X-Hub-Signature-256, segredo o app secret). " +
				"Depois aponte a Meta para https://<seu host>/webhooks/whatsapp com o token de verificação " +
				"`whatsapp.VerifyToken(<o app secret>)` — ele sai do próprio segredo, então não há uma terceira " +
				"coisa para guardar. Preencha o `whatsapp.Receber` no app/setup.go: ele roda dentro da " +
				"requisição, então enfileira e volta. E leia a janela de 24 h no comentário do SendText antes de " +
				"prometer resposta para amanhã a alguém.",
		},
	}
}

const waDecl = `// Package whatsapp is the Cloud API reduced to what an application actually
// deals with: a message that arrived, a status that changed, and the two ways
// of answering.
//
// Meta's payload is an envelope inside an envelope —
// entry[].changes[].value.messages[] — and every type of message is a
// different shape inside it. Parse flattens all of it into Message and
// Status, so nothing above this file ever learns the shape of somebody else's
// JSON.
//
// What is deliberately not here is storage. This package hands over the
// normalisation and the contract; where a message is kept, for how long, and
// what answers it is the application's decision.
package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/conexoes"
)

// The two connections this channel reads its credentials from, by name. They
// are names and not ids because an id is generated when somebody saves the
// connection on the screen, and no recipe can know it beforehand.
const (
	// ConexaoEnvio carries the access token: kind "api", auth "bearer", URL
	// the phone number's own base address —
	// https://graph.facebook.com/v21.0/<phone_number_id>.
	ConexaoEnvio = "whatsapp"
	// ConexaoWebhook carries the app secret, the one Meta signs every
	// delivery with: kind "api", auth "header", header X-Hub-Signature-256.
	ConexaoWebhook = "whatsapp-webhook"
)

// VerifyToken is the token Meta echoes back in the handshake of the webhook,
// derived from the app secret with a label of its own.
//
// It is derived and not stored so that there is no third secret to keep in
// sync — and, being derived, it can travel in a query string (and so into an
// access log, which is where Meta puts it) without telling anybody the secret
// it came from. Paste what this returns into the dashboard's "verify token".
func VerifyToken(appSecret string) string {
	m := hmac.New(sha256.New, []byte(appSecret))
	m.Write([]byte("whatsapp:verify"))
	return hex.EncodeToString(m.Sum(nil))
}

// Media is the file part of a message. The bytes are not here: Meta keeps
// them and hands out an id, which is downloaded separately and expires.
type Media struct {
	// ID is the media id, for the download call.
	ID string
	// Mime is what Meta says it is — always check it before trusting a
	// filename.
	Mime string
	// Caption is the text that came with the file, if any.
	Caption string
}

// Message is one thing somebody sent, whatever its shape on the wire.
type Message struct {
	// ID is the provider's message id (wamid.…). It is stable across the
	// retries of one delivery, which is what makes Dedup possible.
	ID string
	// From is the sender's phone number, digits only, no plus.
	From string
	// Kind is the type: text, image, audio, video, document, sticker,
	// location, reaction, edit — or whatever else Meta invents, passed
	// through as it came rather than collapsed into "unknown".
	Kind string
	// Text is the body of a text message, the emoji of a reaction, or
	// "lat,lon" for a location — the two numbers with a comma between, the
	// way a link to a map wants them.
	Text string
	// Media is the file, when there is one.
	Media *Media
	// ReplyTo is the message this one answers: the quoted message of a reply,
	// the reacted-to message of a reaction, or the replaced message of an
	// edit.
	ReplyTo string
	// At is when the sender sent it.
	At time.Time
}

// Status is what happened to a message this application sent: it left, it
// arrived, it was read, it failed.
type Status struct {
	// ID is the id of the message this is about — the one SendText returned.
	ID string
	// Status is sent, delivered, read or failed.
	Status string
	// Recipient is who it was going to.
	Recipient string
	// At is when the change happened.
	At time.Time
	// Error is what Meta said when Status is "failed"; empty otherwise.
	Error string
}

// Parse turns one delivery into what this application deals with: the
// messages somebody sent, and the statuses of what this application sent.
//
// A delivery can carry both, or neither — Meta sends notifications nobody
// subscribed to — and an empty result is not an error. What is an error is
// JSON that does not parse, which is the only case where the answer should
// not be a 200.
func Parse(body []byte) ([]Message, []Status, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, fmt.Errorf("whatsapp: parsing the delivery: %w", err)
	}
	var msgs []Message
	var sts []Status
	for _, entry := range env.Entry {
		for _, ch := range entry.Changes {
			for _, raw := range ch.Value.Messages {
				msgs = append(msgs, raw.normalize())
			}
			for _, raw := range ch.Value.Statuses {
				sts = append(sts, raw.normalize())
			}
		}
	}
	return msgs, sts, nil
}

// The tags below are only where Meta's name and Go's differ: encoding/json
// matches the rest by name, ignoring case, and a tag that repeats the field
// name is one more thing to keep in sync for nothing.
type envelope struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []rawMessage
				Statuses []rawStatus
			}
		}
	}
}

type rawMessage struct {
	From      string
	ID        string
	Timestamp string
	Type      string
	Context   *struct {
		From string
		ID   string
	}
	Text *struct {
		Body string
	}
	Image    *rawMedia
	Audio    *rawMedia
	Video    *rawMedia
	Document *rawMedia
	Sticker  *rawMedia
	Location *struct {
		Latitude  float64
		Longitude float64
		Name      string
		Address   string
	}
	Reaction *struct {
		MessageID string ` + "`json:\"message_id\"`" + `
		Emoji     string
	}
	Edit *struct {
		OriginalMessageID string ` + "`json:\"original_message_id\"`" + `
		Message           *rawMessage
	}
}

type rawMedia struct {
	ID       string
	MimeType string ` + "`json:\"mime_type\"`" + `
	Caption  string
	Filename string
	Voice    bool
}

type rawStatus struct {
	ID          string
	Status      string
	Timestamp   string
	RecipientID string ` + "`json:\"recipient_id\"`" + `
	Errors      []struct {
		Title   string
		Message string
	}
}

func (r rawMessage) normalize() Message {
	m := Message{ID: r.ID, From: r.From, Kind: r.Type, At: segundos(r.Timestamp)}
	if r.Context != nil {
		m.ReplyTo = r.Context.ID
	}
	switch r.Type {
	case "text":
		if r.Text != nil {
			m.Text = r.Text.Body
		}
	case "image":
		m.Media = midia(r.Image)
	case "audio":
		m.Media = midia(r.Audio)
	case "video":
		m.Media = midia(r.Video)
	case "sticker":
		m.Media = midia(r.Sticker)
	case "document":
		m.Media = midia(r.Document)
		if r.Document != nil && m.Media.Caption == "" {
			m.Media.Caption = r.Document.Filename
		}
	case "location":
		if r.Location != nil {
			m.Text = strconv.FormatFloat(r.Location.Latitude, 'f', -1, 64) + "," +
				strconv.FormatFloat(r.Location.Longitude, 'f', -1, 64)
		}
	case "reaction":
		if r.Reaction != nil {
			m.Text, m.ReplyTo = r.Reaction.Emoji, r.Reaction.MessageID
		}
	case "edit":
		// An edit is a message of its own — its own id, so it is not
		// swallowed by the dedup — carrying the new content and pointing at
		// what it replaces. The application decides whether that means
		// updating a row or keeping both.
		if r.Edit != nil {
			if inner := r.Edit.Message; inner != nil {
				novo := inner.normalize()
				m.Text, m.Media = novo.Text, novo.Media
			}
			m.ReplyTo = r.Edit.OriginalMessageID
		}
	}
	return m
}

func (r rawStatus) normalize() Status {
	s := Status{ID: r.ID, Status: r.Status, Recipient: r.RecipientID, At: segundos(r.Timestamp)}
	if len(r.Errors) > 0 {
		s.Error = strings.TrimSpace(r.Errors[0].Title + " " + r.Errors[0].Message)
	}
	return s
}

func midia(r *rawMedia) *Media {
	if r == nil {
		return &Media{}
	}
	return &Media{ID: r.ID, Mime: r.MimeType, Caption: r.Caption}
}

func segundos(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}

// Dedup is what keeps a provider's retry from becoming two messages. Meta
// promises at least once; exactly once is what the receiver decides, and it
// decides it with the message id.
type Dedup interface {
	// Seen records the id and reports whether it had already been recorded.
	// An empty id is never seen: there is nothing to remember it by.
	Seen(id string) bool
}

// Memory is a Dedup that lives in this process, bounded to max ids (zero is
// ten thousand), oldest first.
//
// Bounded because a set that only grows is a leak with a slow fuse, and in
// this process because a restart in the middle of a retry costing one
// duplicate is cheaper than a table. When it is not — when the duplicate is
// an invoice — the application implements Dedup over its own storage, which
// is why this is an interface.
func Memory(max int) Dedup {
	if max <= 0 {
		max = 10000
	}
	return &memoria{max: max, vistos: make(map[string]struct{}, max)}
}

type memoria struct {
	mu     sync.Mutex
	max    int
	vistos map[string]struct{}
	ordem  []string
}

func (m *memoria) Seen(id string) bool {
	if id == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vistos[id]; ok {
		return true
	}
	m.vistos[id] = struct{}{}
	m.ordem = append(m.ordem, id)
	if len(m.ordem) > m.max {
		delete(m.vistos, m.ordem[0])
		m.ordem = m.ordem[1:]
	}
	return false
}

// Vistas is the deduplication the route uses. Replace it in app/setup.go with
// one backed by your own storage when a duplicate costs more than it costs
// here.
var Vistas Dedup = Memory(0)

// Receber is what this application does with each new message. Fill it in at
// app/setup.go.
//
// It runs inside the request, before the 200 — so it enqueues and returns.
// A receiver that finishes the work before replying is a receiver the sender
// considers down, and then retries, and then the work has happened twice.
var Receber = func(ctx context.Context, m Message) error { return nil }

// ReceberStatus is the same for the status of a message this application
// sent. Empty on purpose: most applications only care once something failed.
var ReceberStatus = func(ctx context.Context, s Status) error { return nil }

// AppSecret is the app secret, out of the sealed connection.
//
// This is the one place in the application that reveals a connection's
// secret, and it is deliberate: an HMAC of the request body is the one thing
// Connections cannot do on the application's behalf.
func AppSecret(c *trilha.Ctx) (string, error) {
	conn, err := conexao(c, ConexaoWebhook)
	if err != nil {
		return "", err
	}
	return conn.Secret.Reveal(), nil
}

// Client is the sending side of the Cloud API.
type Client struct {
	// URL is the base address of the phone number — the connection's own
	// URL, so it is not typed twice.
	URL string
	// HTTP is the connection's client: the access token is put on by its
	// transport and never copied into this package, and the request only
	// leaves for that host.
	HTTP *http.Client
}

// NewClient builds the sending side from the connection that carries the
// token.
func NewClient(c *trilha.Ctx) (*Client, error) {
	conn, err := conexao(c, ConexaoEnvio)
	if err != nil {
		return nil, err
	}
	cli, err := conexoes.Conexoes.Client(c, conn.ID)
	if err != nil {
		return nil, err
	}
	return &Client{URL: conn.URL, HTTP: cli}, nil
}

func conexao(c *trilha.Ctx, nome string) (trilha.Connection, error) {
	lista, err := conexoes.Conexoes.List(c)
	if err != nil {
		return trilha.Connection{}, err
	}
	for _, conn := range lista {
		if strings.EqualFold(conn.Name, nome) {
			return conn, nil
		}
	}
	return trilha.Connection{}, errors.New("whatsapp: there is no connection named " + nome)
}

// SendText sends plain text and answers with the id of what was sent.
//
// **The 24-hour window.** A business may only send free-form text to somebody
// within 24 hours of that person's last message. Outside it Meta refuses the
// call — error 131047 — and the only thing that gets through is SendTemplate,
// with a template approved beforehand. The window is not a rate limit, it is
// the product: every design that says "we will get back to you tomorrow" goes
// out as a template, and finding that out in production is what this comment
// exists to prevent.
func (cl *Client) SendText(ctx context.Context, to, text string) (string, error) {
	return cl.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text":              map[string]any{"preview_url": false, "body": text},
	})
}

// SendTemplate sends a template Meta approved, with its body parameters in
// order, and answers with the id of what was sent.
//
// This is the one that works outside the 24-hour window of SendText — and the
// only one that does. lang is the template's language code, "pt_BR" or
// "en_US"; it has to be the one the template was approved in, or the call is
// refused for a template that does exist.
func (cl *Client) SendTemplate(ctx context.Context, to, name, lang string, params []string) (string, error) {
	tmpl := map[string]any{"name": name, "language": map[string]any{"code": lang}}
	if len(params) > 0 {
		valores := make([]map[string]any, 0, len(params))
		for _, p := range params {
			valores = append(valores, map[string]any{"type": "text", "text": p})
		}
		corpo := map[string]any{"type": "body", "parameters": valores}
		tmpl["components"] = []map[string]any{corpo}
	}
	return cl.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "template",
		"template":          tmpl,
	})
}

func (cl *Client) send(ctx context.Context, payload map[string]any) (string, error) {
	corpo, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(cl.URL, "/")+"/messages", bytes.NewReader(corpo))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := cl.HTTP
	if cli == nil {
		// Only useful in a test: without the connection's client there is no
		// token on the request. There is always a deadline, though.
		cli = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	// Bounded: what comes back is somebody else's, and a body without a limit
	// is a memory limit somebody else sets.
	lido, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var resposta struct {
		Messages []struct{ ID string }
		Error    struct {
			Message string
			Code    int
		}
	}
	_ = json.Unmarshal(lido, &resposta)
	if res.StatusCode >= 400 {
		if resposta.Error.Message != "" {
			return "", fmt.Errorf("whatsapp: %s (code %d)", resposta.Error.Message, resposta.Error.Code)
		}
		return "", fmt.Errorf("whatsapp: the provider answered %d", res.StatusCode)
	}
	if len(resposta.Messages) == 0 {
		return "", errors.New("whatsapp: the provider answered without a message id")
	}
	return resposta.Messages[0].ID, nil
}
`

const waDeclTest = `package whatsapp

import (
	"testing"
	"time"
)

// Os payloads são a forma real da Cloud API: entry[].changes[].value com
// messages[] e statuses[]. Tudo num envelope só porque é um array — e porque
// é assim que se prova que o achatamento funciona.
const entregaDeMensagens = ` + "`" + `{
  "object": "whatsapp_business_account",
  "entry": [
    {
      "id": "102290129340398",
      "changes": [
        {
          "field": "messages",
          "value": {
            "messaging_product": "whatsapp",
            "metadata": {"display_phone_number": "15550783881", "phone_number_id": "106540352242922"},
            "contacts": [{"profile": {"name": "Test Name"}, "wa_id": "972987654321"}],
            "messages": [
              {
                "from": "972987654321",
                "id": "wamid.texto",
                "timestamp": "1697043223",
                "text": {"body": "Body Text"},
                "type": "text"
              },
              {
                "context": {"from": "972123456789", "id": "wamid.citada"},
                "from": "972987654321",
                "id": "wamid.resposta",
                "timestamp": "1697044746",
                "text": {"body": "replied text"},
                "type": "text"
              },
              {
                "context": {"forwarded": true},
                "from": "972987654321",
                "id": "wamid.audio",
                "timestamp": "1697043509",
                "type": "audio",
                "audio": {
                  "mime_type": "audio/ogg; codecs=opus",
                  "sha256": "E3dxS/PdYZE7ppA3pQ4mpCFaXBJ8pX4SRN=",
                  "id": "1234567890987654321",
                  "voice": false
                }
              },
              {
                "from": "972987654321",
                "id": "wamid.imagem",
                "timestamp": "1697043379",
                "type": "image",
                "image": {"mime_type": "image/jpeg", "sha256": "4654+8g=", "id": "65463453"}
              },
              {
                "context": {"forwarded": true},
                "from": "972987654321",
                "id": "wamid.documento",
                "timestamp": "1697043666",
                "type": "document",
                "document": {
                  "caption": "caption",
                  "filename": "filename.pdf",
                  "mime_type": "application/pdf",
                  "sha256": "grwfwe/ZPx0wAbdfbdeFUNItPZ0=",
                  "id": "1234567890987654322"
                }
              },
              {
                "context": {"forwarded": true},
                "from": "972987654321",
                "id": "wamid.local",
                "timestamp": "1697044557",
                "location": {"latitude": 12.25089, "longitude": 43.90539},
                "type": "location"
              },
              {
                "from": "972987654321",
                "id": "wamid.reacao",
                "timestamp": "1697044456",
                "type": "reaction",
                "reaction": {"message_id": "wamid.reagida", "emoji": "😮"}
              },
              {
                "from": "972987654321",
                "id": "wamid.edicao",
                "timestamp": "1749854575",
                "type": "edit",
                "edit": {
                  "original_message_id": "wamid.original",
                  "message": {
                    "context": {"id": "M0"},
                    "type": "image",
                    "image": {
                      "caption": "Updated image caption",
                      "mime_type": "image/jpeg",
                      "sha256": "a1b2c3d4e5f6",
                      "id": "1234567890"
                    }
                  }
                }
              }
            ]
          }
        }
      ]
    }
  ]
}` + "`" + `

const entregaDeStatus = ` + "`" + `{
  "object": "whatsapp_business_account",
  "entry": [
    {
      "id": "5467539754836534",
      "changes": [
        {
          "field": "messages",
          "value": {
            "messaging_product": "whatsapp",
            "metadata": {"display_phone_number": "972123456789", "phone_number_id": "1122334455667"},
            "statuses": [
              {
                "id": "wamid.enviada",
                "status": "delivered",
                "timestamp": "1698266945",
                "recipient_id": "972987654321",
                "conversation": {"id": "7270432eae37498234f4444b35ffc5", "origin": {"type": "service"}},
                "pricing": {"billable": true, "pricing_model": "CBP", "category": "service"}
              }
            ]
          }
        }
      ]
    }
  ]
}` + "`" + `

// Cada tipo vira o mesmo Message, e é isso que faz o resto do app não
// conhecer a forma do JSON de outra pessoa.
func TestParseNormaliza(t *testing.T) {
	msgs, sts, err := Parse([]byte(entregaDeMensagens))
	if err != nil {
		t.Fatal(err)
	}
	if len(sts) != 0 {
		t.Fatalf("status numa entrega de mensagens: %v", sts)
	}
	if len(msgs) != 8 {
		t.Fatalf("mensagens = %d", len(msgs))
	}
	casos := []struct {
		id, kind, texto, replyTo, midia, mime string
	}{
		{id: "wamid.texto", kind: "text", texto: "Body Text"},
		{id: "wamid.resposta", kind: "text", texto: "replied text", replyTo: "wamid.citada"},
		{id: "wamid.audio", kind: "audio", midia: "1234567890987654321", mime: "audio/ogg; codecs=opus"},
		{id: "wamid.imagem", kind: "image", midia: "65463453", mime: "image/jpeg"},
		{id: "wamid.documento", kind: "document", midia: "1234567890987654322", mime: "application/pdf"},
		{id: "wamid.local", kind: "location", texto: "12.25089,43.90539"},
		{id: "wamid.reacao", kind: "reaction", replyTo: "wamid.reagida"},
		{id: "wamid.edicao", kind: "edit", replyTo: "wamid.original", midia: "1234567890", mime: "image/jpeg"},
	}
	for i, quero := range casos {
		got := msgs[i]
		if got.ID != quero.id || got.Kind != quero.kind {
			t.Fatalf("%d: %+v", i, got)
		}
		if got.From != "972987654321" {
			t.Errorf("%s: remetente %q", quero.id, got.From)
		}
		if quero.texto != "" && got.Text != quero.texto {
			t.Errorf("%s: texto %q", quero.id, got.Text)
		}
		if got.ReplyTo != quero.replyTo {
			t.Errorf("%s: replyTo %q", quero.id, got.ReplyTo)
		}
		if quero.midia == "" {
			continue
		}
		if got.Media == nil || got.Media.ID != quero.midia || got.Media.Mime != quero.mime {
			t.Errorf("%s: mídia %+v", quero.id, got.Media)
		}
	}
	// O horário é o do remetente, em segundos, e não o da chegada.
	if quero := time.Unix(1697043223, 0).UTC(); !msgs[0].At.Equal(quero) {
		t.Errorf("horário = %v", msgs[0].At)
	}
	// A reação carrega o emoji no texto, e a edição, o conteúdo novo.
	if msgs[6].Text == "" {
		t.Error("a reação chegou sem emoji")
	}
	if msgs[7].Media.Caption != "Updated image caption" {
		t.Errorf("a edição não trouxe a legenda nova: %+v", msgs[7].Media)
	}
	// Um documento sem legenda cai para o nome do arquivo; este tem legenda,
	// então é a legenda que fica.
	if msgs[4].Media.Caption != "caption" {
		t.Errorf("legenda do documento = %q", msgs[4].Media.Caption)
	}
}

// Status de entrega é a outra metade do mesmo envelope.
func TestParseStatus(t *testing.T) {
	msgs, sts, err := Parse([]byte(entregaDeStatus))
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("mensagem numa entrega de status: %v", msgs)
	}
	if len(sts) != 1 {
		t.Fatalf("status = %d", len(sts))
	}
	s := sts[0]
	if s.ID != "wamid.enviada" || s.Status != "delivered" || s.Recipient != "972987654321" {
		t.Fatalf("%+v", s)
	}
	if !s.At.Equal(time.Unix(1698266945, 0).UTC()) {
		t.Errorf("horário = %v", s.At)
	}
}

// Uma entrega que a Meta manda sem nada dentro não é erro: é um 200 e nenhuma
// mensagem. Erro é JSON que não é JSON.
func TestParseVazioEQuebrado(t *testing.T) {
	msgs, sts, err := Parse([]byte(` + "`" + `{"object":"whatsapp_business_account","entry":[]}` + "`" + `))
	if err != nil || len(msgs) != 0 || len(sts) != 0 {
		t.Fatalf("%v %v %v", msgs, sts, err)
	}
	if _, _, err := Parse([]byte("nada disso")); err == nil {
		t.Fatal("JSON quebrado passou")
	}
}

// O mesmo id duas vezes é uma mensagem só, e a memória não cresce para
// sempre: o mais antigo sai quando o limite estoura.
func TestMemoryDedup(t *testing.T) {
	d := Memory(2)
	if d.Seen("a") {
		t.Fatal("o primeiro já era visto")
	}
	if !d.Seen("a") {
		t.Fatal("o repetido passou")
	}
	d.Seen("b")
	d.Seen("c") // "a" sai aqui
	if d.Seen("a") {
		t.Fatal("o limite não descartou o mais antigo")
	}
	// Sem id não há o que lembrar, e isso não é "visto".
	if d.Seen("") || d.Seen("") {
		t.Fatal("um id vazio virou visto")
	}
}
`

const waRoute = `// Package whatsapp is the address Meta calls: the handshake that proves this
// endpoint is yours, and the deliveries.
//
// Two things make this different from a screen. It is an API — kind.go says
// so — which is what keeps CSRF out of the way: the client here is Meta, not
// a form of this site, and a third party cannot carry a token of ours. And
// its address is fixed, because it is typed into somebody else's dashboard;
// moving it means going back there.
package whatsapp

import (
	"crypto/subtle"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"

	wa "{{.Module}}/internal/whatsapp"
)

// GET is the handshake: Meta calls this address once with a challenge and a
// token, and only subscribes if the challenge comes back unchanged.
func GET(c *trilha.Ctx) error {
	segredo, err := wa.AppSecret(c)
	if err != nil {
		return trilha.Errorf(http.StatusForbidden, "webhook não configurado")
	}
	esperado := wa.VerifyToken(segredo)
	// Constant time, because a comparison that returns on the first wrong
	// byte is a comparison somebody can measure.
	if c.Query("hub.mode") != "subscribe" ||
		subtle.ConstantTimeCompare([]byte(c.Query("hub.verify_token")), []byte(esperado)) != 1 {
		return trilha.Errorf(http.StatusForbidden, "token de verificação inválido")
	}
	return c.Text(http.StatusOK, c.Query("hub.challenge"))
}

// POST is a delivery: check the signature, normalise, refuse what has already
// been seen, hand it over, answer.
//
// The order is the part people get wrong. Receber runs before the 200 and is
// expected to enqueue and return — a receiver that finishes the work first is
// a receiver Meta considers down, and then retries, and then the work has
// happened twice.
func POST(c *trilha.Ctx) error {
	segredo, err := wa.AppSecret(c)
	if err != nil {
		return recusa()
	}
	corpo, err := webhook.VerifyHMAC(c.Request(), webhook.HMACOpts{Secret: segredo})
	if err != nil {
		return recusa()
	}
	msgs, sts, err := wa.Parse(corpo)
	if err != nil {
		return trilha.Errorf(http.StatusBadRequest, "corpo inválido")
	}
	for _, m := range msgs {
		// The provider promises at least once. This line is where that
		// becomes exactly once.
		if wa.Vistas.Seen(m.ID) {
			continue
		}
		if err := wa.Receber(c.Context(), m); err != nil {
			c.Log().Error("whatsapp: receiving a message", "id", m.ID, "err", err)
		}
	}
	for _, s := range sts {
		if err := wa.ReceberStatus(c.Context(), s); err != nil {
			c.Log().Error("whatsapp: receiving a status", "id", s.ID, "err", err)
		}
	}
	return c.Text(http.StatusOK, "ok")
}

// recusa is one answer for every way of not being Meta: no signature, the
// wrong one, a body that changed, a body over the limit, no connection
// configured. Saying which one tells a stranger how close they got.
func recusa() error {
	return trilha.Errorf(http.StatusUnauthorized, "assinatura inválida")
}
`

const waKind = `package whatsapp

import "github.com/emersonjoe/trilha"

// This really is an API, and saying so out loud is what keeps CSRF off this
// route: the client is Meta, which cannot carry a token of this site, and the
// signature in the body's HMAC is what stands in its place. Errors go out as
// JSON for the same reason — there is nobody here to read an HTML page.
var Kind = trilha.KindAPI
`

const waTest = `package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/conexoes"
	"{{.Module}}/internal/whatsapp"
)

const segredoDoAppDeTeste = "um-app-secret-de-teste"

func assinaWhatsApp(corpo string) string {
	m := hmac.New(sha256.New, []byte(segredoDoAppDeTeste))
	m.Write([]byte(corpo))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

const entregaDeTeste = ` + "`" + `{"object":"whatsapp_business_account","entry":[{"id":"1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","messages":[{"from":"5511999999999","id":"wamid.umaso","timestamp":"1697043223","type":"text","text":{"body":"oi"}}]}}]}]}` + "`" + `

// A rota recusa o que não está assinado, aceita o que está, e conta a mesma
// mensagem uma vez só: o reenvio do provedor não vira duas mensagens.
func TestWebhookDoWhatsApp(t *testing.T) {
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cli := trilha.NewTestClient(t, newApp())

	// O segredo mora selado na conexão, e é de lá que a rota o lê.
	if _, err := conexoes.Conexoes.Save(nil, trilha.Connection{
		Kind: "api", Name: whatsapp.ConexaoWebhook,
		URL: "https://graph.facebook.com/v21.0/1", Auth: "header",
		Header: "X-Hub-Signature-256", Secret: trilha.Secret(segredoDoAppDeTeste),
	}); err != nil {
		t.Fatal(err)
	}

	var recebidas []whatsapp.Message
	whatsapp.Receber = func(_ context.Context, m whatsapp.Message) error {
		recebidas = append(recebidas, m)
		return nil
	}

	entrega := func(assinatura string) *trilha.TestResponse {
		return cli.Request(http.MethodPost, "/webhooks/whatsapp",
			trilha.WithBody("application/json", entregaDeTeste),
			trilha.WithHeader("X-Hub-Signature-256", assinatura))
	}

	// Assinatura errada e corpo alterado caem no mesmo 401, sem dizer qual.
	entrega("sha256=00").WantStatus(http.StatusUnauthorized)
	entrega(assinaWhatsApp(entregaDeTeste + " ")).WantStatus(http.StatusUnauthorized)
	if len(recebidas) != 0 {
		t.Fatalf("passou sem assinatura: %v", recebidas)
	}

	// Assinada, ela chega — e o reenvio da mesma entrega não chega de novo.
	entrega(assinaWhatsApp(entregaDeTeste)).WantStatus(http.StatusOK)
	entrega(assinaWhatsApp(entregaDeTeste)).WantStatus(http.StatusOK)
	if len(recebidas) != 1 {
		t.Fatalf("o mesmo message_id virou %d mensagens", len(recebidas))
	}
	if recebidas[0].Text != "oi" || recebidas[0].From != "5511999999999" {
		t.Fatalf("%+v", recebidas[0])
	}

	// E o aperto de mão: o desafio volta só para quem sabe o token.
	token := whatsapp.VerifyToken(segredoDoAppDeTeste)
	cli.Get("/webhooks/whatsapp?hub.mode=subscribe&hub.challenge=1234&hub.verify_token=" + token).
		WantStatus(http.StatusOK).WantContains("1234")
	cli.Get("/webhooks/whatsapp?hub.mode=subscribe&hub.challenge=1234&hub.verify_token=errado").
		WantStatus(http.StatusForbidden)
}
`
