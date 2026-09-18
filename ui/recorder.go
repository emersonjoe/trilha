package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// RecorderOpts describes the capture island: where the recording goes and how
// long it may be.
type RecorderOpts struct {
	// Action is the route that receives the recording, as any form's action.
	Action string
	// Name is the form field the route reads with c.File; default "audio".
	Name string
	// MaxSeconds stops the recording on its own. Zero leaves it to the
	// person, and to the route's own size limit.
	MaxSeconds int
	// Mime is what MediaRecorder is asked for, when the browser supports it;
	// default "audio/webm".
	Mime string
	// Label is the text of the record button; the default comes from the
	// request's locale ("Record" / "Gravar").
	Label string
	// Target is the id of the element the answer swaps, as in UploadTo. Empty
	// leaves a plain form: the route answers the whole page.
	Target string
	Attrs  []h.Node
}

// Recorder is the microphone half of ui.Audio: a form that records at the
// counter and posts the recording to a route, with the same progress bar the
// upload has. With ui.RecorderScript and ui.UploadScript on the page, the
// button records with MediaRecorder and the form is sent by XHR, so the answer
// swaps Target; with JavaScript off, denied or unsupported, what is left is an
// <input type="file" accept="audio/*" capture> — on a phone, the recorder of
// the system — and the form posts as any other form.
//
//	ui.Recorder(c, ui.RecorderOpts{Action: "/api/voice", Target: "answer", MaxSeconds: 60})
//	ui.RecorderScript(c)
//	ui.UploadScript(c)
//
// The route reads one file (c.File with FileRules) and answers the fragment.
// The browser only gives the microphone over HTTPS and with a
// Permissions-Policy that allows it: the framework's default denies it, so an
// app that records sets Config.Security.PermissionsPolicy with
// "microphone=(self)" in it.
func Recorder(c *trilha.Ctx, o RecorderOpts) h.Node {
	pt := langOf(c) == "pt-BR"
	if o.Name == "" {
		o.Name = "audio"
	}
	if o.Mime == "" {
		o.Mime = "audio/webm"
	}
	if o.Label == "" {
		o.Label = word(pt, "Record", "Gravar")
	}
	form := []h.Node{
		h.Class("ui-recorder"),
		h.Method("post"), h.Enctype("multipart/form-data"),
		h.Data("ui-recorder", ""),
		h.Data("ui-recorder-mime", o.Mime),
		// The script says the other words too: a label inside it would be a
		// sentence in English on a page in Portuguese.
		h.Data("ui-recorder-stop", word(pt, "Stop", "Parar")),
		h.Data("ui-recorder-record", o.Label),
		h.Data("ui-recorder-denied", word(pt,
			"The microphone was not allowed. Choose or record a file instead.",
			"O microfone não foi liberado. Escolha ou grave um arquivo.")),
	}
	if o.Action != "" {
		form = append(form, h.Action(o.Action))
	}
	if o.Target != "" {
		form = append(form, UploadTo(o.Target))
	}
	if o.MaxSeconds > 0 {
		form = append(form, h.Data("ui-recorder-max", strconv.Itoa(o.MaxSeconds)))
	}
	form = append(form, o.Attrs...)
	if c != nil {
		form = append(form, trilha.CSRFInput(c))
	}
	form = append(form,
		// The fallback comes first and is real: it is what a browser with no
		// MediaRecorder, or a person who said no to the microphone, uses.
		Input(h.Type("file"), h.Name(o.Name), h.Accept("audio/*"), h.Attr("capture", ""),
			h.Data("ui-recorder-file", ""), h.Aria("label", o.Label)),
		// Hidden until the script finds a microphone: a button that cannot
		// record is worse than no button.
		Button(h.Data("ui-recorder-toggle", ""), h.Hidden(), h.Aria("pressed", "false"), h.Text(o.Label)),
		h.Output(h.Class("ui-recorder-time"), h.Data("ui-recorder-time", ""), h.Aria("live", "off"), h.Text("0:00")),
		h.P(h.Class("ui-recorder-note"), h.Data("ui-recorder-note", ""), h.Role("status"), h.Hidden()),
		UploadBar(),
		Submit(h.Text(word(pt, "Send", "Enviar"))),
	)
	return h.Form(form...)
}

// RecorderScript loads ui.recorder.js, the behaviour behind Recorder: the
// microphone, the elapsed time and the recording put into the file field. Put
// it once on the page that records — Head does not load it, so a page without
// a recorder downloads nothing. The upload with progress is still
// UploadScript's, and a page that swaps the answer loads both.
func RecorderScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.recorder.js")), h.Defer())
}
