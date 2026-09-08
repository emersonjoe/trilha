package editor

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Status is what the editor may be doing.
type Status string

// Author is who is writing.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Revision points at the one before it.
type Revision struct {
	At   time.Time `json:"at"`
	Prev *Revision `json:"prev,omitempty"`
}

// EditorProps is everything the editor needs to take over the textarea.
type EditorProps struct {
	Author
	// WPM is the reading speed shown under the counter.
	WPM      int               `json:"wpm"`
	Status   Status            `json:"status"`
	Tags     []string          `json:"tags"`
	Draft    *Revision         `json:"draft,omitempty"`
	Counts   map[string]int    `json:"counts"`
	Raw      []byte            `json:"raw"`
	Secret   string            `json:"-"`
	internal bool
	Size     struct {
		Rows int `json:"rows"`
	} `json:"size"`
}

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(c.Island("/editor.js", EditorProps{WPM: 200}, h.Textarea())), nil
}
