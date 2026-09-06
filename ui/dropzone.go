package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha/h"
)

// DropzoneOpts describes the area. Name is the form field, and it is the name
// the route reads with c.Files.
type DropzoneOpts struct {
	Name string
	// Accept and MaxSize are the courtesy check in the browser: they keep the
	// obvious mistake from crossing the network. The answer is still the
	// route's, in c.Files(trilha.FileRules{…}), which is the only place that
	// reads what is inside the file.
	Accept  string // the accept attribute: "application/pdf,image/*"
	MaxSize int64  // bytes; zero leaves it to the route and the body limit
	// Single turns off multiple selection; the area takes one file at a time.
	Single bool
	Attrs  []h.Node
}

// Dropzone is an area that takes files by dragging or by clicking, inside the
// form that says where they go. With ui.UploadScript on the page the files are
// sent one request each — one line of the queue, one progress bar, one message
// each — and every answer swaps the element ui.UploadTo names. With the script
// off it is an <input type=file multiple> and the form's own button sends them
// all at once, which is why the route is written once, against c.Files.
//
//	h.Form(h.Method("post"), h.Action("/files"), h.EncType("multipart/form-data"),
//		ui.UploadTo("list"), trilha.CSRFInput(c),
//		ui.Dropzone(ui.DropzoneOpts{Name: "files", Accept: "application/pdf", MaxSize: 50 << 20},
//			h.P(h.Text("Drop the PDFs here, or click to choose"))),
//		ui.Submit(h.Text("Send")))
//
// A file the route refuses answers 4xx, and the queue shows the answer's text
// on that file's line: the message belongs to the file, not to the form.
func Dropzone(o DropzoneOpts, children ...h.Node) h.Node {
	id := o.Name
	input := []h.Node{h.ID(id), h.Name(o.Name), h.Type("file")}
	if !o.Single {
		input = append(input, h.Attr("multiple", ""))
	}
	if o.Accept != "" {
		input = append(input, h.Accept(o.Accept))
	}
	box := []h.Node{h.Class("ui-dropzone"), h.Data("ui-dropzone", "")}
	if o.MaxSize > 0 {
		box = append(box, h.Data("ui-dropzone-max", strconv.FormatInt(o.MaxSize, 10)))
	}
	box = append(box, o.Attrs...)
	return h.Div(append(box,
		h.Label(append([]h.Node{h.Class("ui-dropzone-area"), h.For(id)}, children...)...),
		h.Input(input...),
		h.Ul(h.Class("ui-queue"), h.Data("ui-queue", "")),
	)...)
}
