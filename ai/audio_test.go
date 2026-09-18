package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAudio answers like the audio half of the protocol: /audio/transcriptions
// takes the multipart and /audio/speech takes the JSON. It keeps what arrived,
// which is the half of the contract the tests are about.
type fakeAudio struct {
	t      *testing.T
	srv    *httptest.Server
	status int

	// what the last request carried
	path     string
	auth     string
	file     []byte
	filename string
	fields   map[string]string
	speak    map[string]any
	calls    int
}

func newFakeAudio(t *testing.T) *fakeAudio {
	f := &fakeAudio{t: t, fields: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls++
		f.path = r.URL.Path
		f.auth = r.Header.Get("Authorization")
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error":{"message":"scripted failure","type":"server_error","code":"boom"}}`))
			return
		}
		switch r.URL.Path {
		case "/v1/audio/transcriptions":
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			file, hdr, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			defer file.Close()
			f.file, _ = io.ReadAll(file)
			f.filename = hdr.Filename
			for k, v := range r.MultipartForm.Value {
				f.fields[k] = v[0]
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"bom dia","language":"portuguese","duration":2.5}`))
		case "/v1/audio/speech":
			f.speak = map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&f.speak); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3fake-mp3-bytes"))
		default:
			http.Error(w, "not found", 404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAudio) client() *Client {
	return &Client{BaseURL: f.srv.URL + "/v1", APIKey: "sk-test", Model: "fake-1"}
}

// #269 — the recording travels as a file, and the fields beside it are the
// ones the protocol names.
func TestTranscribeSendsTheFileAndTheFields(t *testing.T) {
	f := newFakeAudio(t)
	got, err := f.client().Transcribe(context.Background(), strings.NewReader("webm-bytes"),
		TranscribeOpts{Language: "pt", Prompt: "Trilha", Format: "verbose_json", Filename: "balcao.webm"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "bom dia" || got.Language != "portuguese" || got.Duration != 2.5 {
		t.Fatalf("transcript = %+v", got)
	}
	if f.path != "/v1/audio/transcriptions" || f.auth != "Bearer sk-test" {
		t.Fatalf("path %q auth %q", f.path, f.auth)
	}
	if string(f.file) != "webm-bytes" || f.filename != "balcao.webm" {
		t.Fatalf("file %q named %q", f.file, f.filename)
	}
	for k, want := range map[string]string{"model": "whisper-1", "language": "pt", "prompt": "Trilha", "response_format": "verbose_json"} {
		if f.fields[k] != want {
			t.Fatalf("field %s = %q, want %q", k, f.fields[k], want)
		}
	}
}

// The defaults are a whole call on their own: the model from the environment,
// the json format, the name ui.Recorder sends.
func TestTranscribeDefaults(t *testing.T) {
	f := newFakeAudio(t)
	t.Setenv("TRILHA_AI_TRANSCRIBE_MODEL", "whisper-large-v3")
	if _, err := f.client().Transcribe(context.Background(), strings.NewReader("x"), TranscribeOpts{}); err != nil {
		t.Fatal(err)
	}
	if f.fields["model"] != "whisper-large-v3" || f.fields["response_format"] != "json" || f.filename != "audio.webm" {
		t.Fatalf("defaults: %v %q", f.fields, f.filename)
	}
	if _, ok := f.fields["language"]; ok {
		t.Fatal("an empty language must not be sent")
	}
}

// A provider that fails is an *Error with its status and its message, like the
// chat half.
func TestTranscribeProviderError(t *testing.T) {
	f := newFakeAudio(t)
	f.status = 500
	_, err := f.client().Transcribe(context.Background(), strings.NewReader("x"), TranscribeOpts{})
	var e *Error
	if !errors.As(err, &e) || e.Status != 500 || e.Message != "scripted failure" || e.Code != "boom" {
		t.Fatalf("error = %v", err)
	}
}

// The limit is checked here, before the bytes cross the network: a recording
// over it never reaches the provider.
func TestTranscribeRefusesAnOversizedRecording(t *testing.T) {
	f := newFakeAudio(t)
	_, err := f.client().Transcribe(context.Background(), strings.NewReader(strings.Repeat("a", 2048)), TranscribeOpts{MaxBytes: 1024})
	var e *Error
	if !errors.As(err, &e) || e.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error = %v", err)
	}
	if f.calls != 0 {
		t.Fatal("the oversized recording reached the provider")
	}
	// And the recording that fits still goes.
	if _, err := f.client().Transcribe(context.Background(), strings.NewReader("aaa"), TranscribeOpts{MaxBytes: 1024}); err != nil {
		t.Fatal(err)
	}
}

func TestTranscribeRefusesAnEmptyRecording(t *testing.T) {
	f := newFakeAudio(t)
	if _, err := f.client().Transcribe(context.Background(), strings.NewReader(""), TranscribeOpts{}); err == nil {
		t.Fatal("an empty recording is not a transcription")
	}
	if f.calls != 0 {
		t.Fatal("the empty recording reached the provider")
	}
}

// #269 — Speak posts the JSON of the protocol and hands back the bytes.
func TestSpeakSendsTheJSONAndReturnsTheAudio(t *testing.T) {
	f := newFakeAudio(t)
	body, err := f.client().Speak(context.Background(), "bom dia", SpeakOpts{Voice: "nova", Speed: 1.2})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil || string(b) != "ID3fake-mp3-bytes" {
		t.Fatalf("audio = %q %v", b, err)
	}
	if f.path != "/v1/audio/speech" || f.auth != "Bearer sk-test" {
		t.Fatalf("path %q auth %q", f.path, f.auth)
	}
	want := map[string]any{"model": "tts-1", "input": "bom dia", "voice": "nova", "response_format": "mp3", "speed": 1.2}
	for k, v := range want {
		if f.speak[k] != v {
			t.Fatalf("%s = %v, want %v", k, f.speak[k], v)
		}
	}
	if got := (SpeakOpts{}).ContentType(); got != "audio/mpeg" {
		t.Fatalf("ContentType = %q", got)
	}
	if got := (SpeakOpts{Format: "opus"}).ContentType(); got != "audio/ogg" {
		t.Fatalf("ContentType(opus) = %q", got)
	}
}

func TestSpeakDefaultsAndRefusals(t *testing.T) {
	f := newFakeAudio(t)
	t.Setenv("TRILHA_AI_SPEECH_MODEL", "gpt-4o-mini-tts")
	body, err := f.client().Speak(context.Background(), "oi", SpeakOpts{})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	if f.speak["model"] != "gpt-4o-mini-tts" || f.speak["voice"] != "alloy" {
		t.Fatalf("defaults: %v", f.speak)
	}
	if _, ok := f.speak["speed"]; ok {
		t.Fatal("a zero speed must not be sent")
	}
	if _, err := f.client().Speak(context.Background(), "   ", SpeakOpts{}); err == nil {
		t.Fatal("empty text is not speech")
	}
	f.status = 502
	_, err = f.client().Speak(context.Background(), "oi", SpeakOpts{})
	var e *Error
	if !errors.As(err, &e) || e.Status != 502 || e.Message != "scripted failure" {
		t.Fatalf("error = %v", err)
	}
}
