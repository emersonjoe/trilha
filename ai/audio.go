package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// The two audio endpoints of the protocol, under BaseURL. Transcription takes
// the recording as multipart/form-data; speech takes JSON and answers with the
// audio bytes.
const (
	transcribePath = "/audio/transcriptions"
	speechPath     = "/audio/speech"
	// defaultTranscribeBytes is the ceiling the protocol documents for one
	// recording.
	defaultTranscribeBytes = 25 << 20
)

// TranscribeOpts says how a recording is transcribed. Every field has a
// default, so the zero value is a valid call.
type TranscribeOpts struct {
	// Model is the transcription model: TRILHA_AI_TRANSCRIBE_MODEL, then
	// "whisper-1".
	Model string
	// Language is the ISO-639-1 code of what was said ("pt", "en"). Empty
	// leaves the detection to the provider.
	Language string
	// Prompt guides the style and spells out the words the model would
	// otherwise get wrong — names, acronyms.
	Prompt string
	// Format is the response_format: "json" (the default), "verbose_json",
	// which also fills Language and Duration, or "text".
	Format string
	// Filename is the name the provider sees; its extension says which
	// container the bytes are in. Default "audio.webm", what ui.Recorder
	// sends.
	Filename string
	// MaxBytes is the most that is read from r; default 25 MB, the ceiling of
	// the protocol. A longer recording answers an *Error with Status 413 and
	// no request leaves the process.
	MaxBytes int64
}

// Transcript is what came back from a transcription. Language and Duration are
// only filled by the "verbose_json" format.
type Transcript struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// Transcribe sends the recording in r to /audio/transcriptions and returns
// what was said. The reader is read to the end — and no further than MaxBytes
// — before the request leaves, so a route can hand it the upload as it came:
//
//	up, err := c.File("audio", trilha.FileRules{MaxSize: 10 << 20, Accept: []string{"audio/*"}})
//	if err != nil {
//		return err
//	}
//	defer up.Close()
//	t, err := client.Transcribe(c, up, ai.TranscribeOpts{Language: "pt", Filename: up.Name})
//
// A refusal from the provider is an *Error with its status and message, as in
// Chat; a recording over the limit is an *Error with status 413.
func (c *Client) Transcribe(ctx context.Context, r io.Reader, o TranscribeOpts) (Transcript, error) {
	if o.Model == "" {
		o.Model = os.Getenv("TRILHA_AI_TRANSCRIBE_MODEL")
	}
	if o.Model == "" {
		o.Model = "whisper-1"
	}
	if o.Format == "" {
		o.Format = "json"
	}
	if o.Filename == "" {
		o.Filename = "audio.webm"
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = defaultTranscribeBytes
	}
	if r == nil {
		return Transcript{}, errors.New("ai: Transcribe of a nil reader")
	}
	// One byte past the limit is enough to know it was crossed, and it is read
	// here so that the oversized recording never reaches the provider — the
	// bill and the round trip are both avoided.
	audio, err := io.ReadAll(io.LimitReader(r, o.MaxBytes+1))
	if err != nil {
		return Transcript{}, fmt.Errorf("ai: reading audio: %w", err)
	}
	if int64(len(audio)) > o.MaxBytes {
		return Transcript{}, &Error{Status: http.StatusRequestEntityTooLarge, Code: "audio_too_large",
			Message: "audio is larger than " + strconv.FormatInt(o.MaxBytes, 10) + " bytes"}
	}
	if len(audio) == 0 {
		return Transcript{}, errors.New("ai: Transcribe of an empty recording")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", o.Filename)
	if err != nil {
		return Transcript{}, err
	}
	if _, err := fw.Write(audio); err != nil {
		return Transcript{}, err
	}
	for _, f := range [][2]string{{"model", o.Model}, {"response_format", o.Format}, {"language", o.Language}, {"prompt", o.Prompt}} {
		if f[1] == "" {
			continue
		}
		if err := mw.WriteField(f[0], f[1]); err != nil {
			return Transcript{}, err
		}
	}
	if err := mw.Close(); err != nil {
		return Transcript{}, err
	}
	hr, err := c.newRequest(ctx, transcribePath, mw.FormDataContentType(), &body)
	if err != nil {
		return Transcript{}, err
	}
	resp, err := c.send(hr)
	if err != nil {
		return Transcript{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Transcript{}, fmt.Errorf("ai: reading transcription: %w", err)
	}
	if o.Format == "text" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return Transcript{Text: strings.TrimSpace(string(raw))}, nil
	}
	var out Transcript
	if err := json.Unmarshal(raw, &out); err != nil {
		return Transcript{}, fmt.Errorf("ai: decoding transcription: %w", err)
	}
	return out, nil
}

// SpeakOpts says how text is turned into speech. The zero value speaks with
// the provider's default voice, as MP3.
type SpeakOpts struct {
	// Model is the speech model: TRILHA_AI_SPEECH_MODEL, then "tts-1".
	Model string
	// Voice is the provider's voice name; default "alloy".
	Voice string
	// Format is the response_format: "mp3" (the default), "opus", "aac",
	// "flac", "wav" or "pcm". ContentType says what to serve it as.
	Format string
	// Speed multiplies the pace (0.25 to 4); zero leaves the provider's 1.
	Speed float64
}

// speechTypes maps the formats of the protocol to the media type a browser
// expects; ui.Audio plays every one of them but pcm.
var speechTypes = map[string]string{
	"mp3": "audio/mpeg", "opus": "audio/ogg", "aac": "audio/aac",
	"flac": "audio/flac", "wav": "audio/wav", "pcm": "audio/pcm",
}

// ContentType is the media type of the audio Speak returns for SpeakOpts.Format
// — "audio/mpeg" for the default MP3 — which is what the route serves it as.
func (o SpeakOpts) ContentType() string {
	if t, ok := speechTypes[strings.ToLower(o.Format)]; ok {
		return t
	}
	if o.Format == "" {
		return "audio/mpeg"
	}
	return "audio/" + strings.ToLower(o.Format)
}

// Speak sends text to /audio/speech and returns the audio as it streams in.
// The caller closes the body; a route hands it to ui.Audio with Ctx.Inline and
// the type of the format:
//
//	body, err := client.Speak(c, answer, ai.SpeakOpts{Voice: "nova"})
//	if err != nil {
//		return err
//	}
//	defer body.Close()
//	c.Header("Cache-Control", "private, no-store")
//	return c.Inline("answer.mp3", body, ai.SpeakOpts{}.ContentType())
//
// Empty text is an error before any request leaves; a refusal from the
// provider is an *Error with its status and message, as in Chat.
func (c *Client) Speak(ctx context.Context, text string, o SpeakOpts) (io.ReadCloser, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("ai: Speak of empty text")
	}
	if o.Model == "" {
		o.Model = os.Getenv("TRILHA_AI_SPEECH_MODEL")
	}
	if o.Model == "" {
		o.Model = "tts-1"
	}
	if o.Voice == "" {
		o.Voice = "alloy"
	}
	if o.Format == "" {
		o.Format = "mp3"
	}
	req := map[string]any{"model": o.Model, "input": text, "voice": o.Voice, "response_format": o.Format}
	if o.Speed > 0 {
		req["speed"] = o.Speed
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hr, err := c.newRequest(ctx, speechPath, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := c.send(hr)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
