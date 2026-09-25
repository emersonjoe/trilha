package ui

// Pattern is one screen the kit already knows how to build, as data: which
// components it takes, a complete page.go that compiles, what the page needs
// to receive, and what keeps it usable with a keyboard and a screen reader.
//
// It exists for whoever writes the next screen — a person or an agent — so the
// question "how do I build a listing here?" is one lookup and not an evening
// of reading demos. `trilha ui patterns --json` prints the list, the MCP tool
// get_pattern answers one, and every snippet is a file in examples/patterns
// that the repository compiles.
type Pattern struct {
	// Name is the address of the pattern: "list-with-filter".
	Name string `json:"name"`
	// Summary is the one sentence that says when to use it.
	Summary string `json:"summary"`
	// Components are the kit symbols the snippet uses, as the catalog of
	// `trilha ui components --json` names them.
	Components []string `json:"components"`
	// Snippet is a complete page.go, at most 60 lines, that compiles.
	Snippet string `json:"snippet"`
	// Data is the contract: what the page needs to receive and where it
	// should come from.
	Data string `json:"data"`
	// A11y are the notes that keep it usable without a mouse or a screen:
	// focus, aria and keyboard.
	A11y []string `json:"a11y"`
}

// Patterns returns the patterns in a stable order. The slice is new on every
// call; changing it changes nothing.
//
//	for _, p := range ui.Patterns() {
//		fmt.Println(p.Name, "—", p.Summary)
//	}
func Patterns() []Pattern {
	out := make([]Pattern, 0, len(patterns))
	for _, p := range patterns {
		p.Components = append([]string(nil), p.Components...)
		p.A11y = append([]string(nil), p.A11y...)
		p.Snippet = patternSnippets[p.Name]
		out = append(out, p)
	}
	return out
}

// patterns is the table; the snippets come from patterns_snippets.go, which
// is generated from examples/patterns.
var patterns = []Pattern{
	{
		Name:       "list-with-filter",
		Summary:    "A listing somebody searches, filters, orders and pages, with all of it in the address.",
		Components: []string{"DataTable", "Columns", "ListState", "Select", "SelectOptions", "Option", "Badge", "Empty", "EmptyOpts", "PageHeader", "Stack"},
		Data: "type Query struct{ trilha.ListParams; Status string `form:\"status\"` } bound by c.Bind; " +
			"Orders(c, q) ([]Order, int, error) answers one page and the total from your store, " +
			"ordering only by the columns marked Sort.",
		A11y: []string{
			"The table has a caption (ListState.Caption) that a screen reader announces.",
			"The filter select carries an aria-label, since it has no visible label.",
			"Sorting and paging are real links: they work with the keyboard and without JavaScript.",
		},
	},
	{
		Name:       "async-form",
		Summary:    "A form that posts without leaving the page and answers a 422 beside the field that is wrong.",
		Components: []string{"Swap", "FormError", "Field", "Input", "InvalidIf", "Errors", "Submit"},
		Data: "type Profile struct with form and validate tags, bound by c.Bind; " +
			"Save(c, p) error stores a valid one. A trilha.FieldErrors from Bind is the 422.",
		A11y: []string{
			"On a 422 the kit swaps the form and moves focus to the first field with aria-invalid.",
			"ui.Field ties the message to the input with aria-describedby.",
			"ui.FormError is the summary a screen reader hears when the answer arrives.",
		},
	},
	{
		Name:       "approval-inbox",
		Summary:    "What waits for a person's decision, with approve and reject and the reason in the same form.",
		Components: []string{"Inbox", "InboxRow", "InboxOpts", "PageHeader", "Stack"},
		Data: "Waiting(c) ([]approval.Record, error), by default the *approval.Approvals Setup provided " +
			"(trilha add approvals), filtered to what the person may decide; Decide checks it again.",
		A11y: []string{
			"Each row's buttons name the request they decide, not only \"Approve\".",
			"A late row says so in text as well as in color.",
			"The reason field has a label and is part of the same form as the buttons.",
		},
	},
	{
		Name:       "dashboard-chart",
		Summary:    "The numbers on top and the drawings beside them, server-rendered SVG with no chart library.",
		Components: []string{"Stat", "SparklineTitle", "SparkOpts", "ChartTitle", "Bars", "Donut", "Datum", "Card", "CardHeader", "CardTitle", "CardContent", "Grid", "Cols"},
		Data: "Load(c) (Numbers, error): the formatted totals, []ui.Datum per category and the weekly " +
			"[]float64 — counted by your store, not by the page.",
		A11y: []string{
			"Every chart carries a ui.ChartTitle, which becomes the SVG's accessible name.",
			"The values are in the chart's text as well, so the numbers are read and not only drawn.",
			"The cards stack to one column on a narrow screen (ui.Cols(1, 2)).",
		},
	},
	{
		Name:       "master-detail",
		Summary:    "A list on one side and the chosen row on the other, the choice in the address.",
		Components: []string{"DataTable", "Columns", "ListState", "Empty", "EmptyOpts", "Card", "CardHeader", "CardTitle", "CardDescription", "CardContent", "Badge", "Grid", "Cols"},
		Data: "type Query struct{ trilha.ListParams; ID string `form:\"id\"` }; Customers(c, q) answers one " +
			"page and the total, Find(c, id) one row or trilha.ErrNotFound.",
		A11y: []string{
			"Each row is a link (ListState.RowHref), so the keyboard reaches the detail.",
			"The detail is a section with an aria-label, a landmark to jump to.",
			"The choice lives in the address: back and reload keep it.",
		},
	},
	{
		Name:       "upload-progress",
		Summary:    "Files sent with a progress bar and checked by content on the server, working without JavaScript too.",
		Components: []string{"UploadTo", "UploadBar", "UploadScript", "Field", "Input", "InvalidIf", "Errors", "Submit", "PageHeader", "Stack"},
		Data: "trilha.FileRules (size, count, accepted types, checked against the bytes); " +
			"Save(c, up) keeps one *trilha.Upload in your blob store.",
		A11y: []string{
			"The progress bar has an aria-label and is a real <progress>, read as a percentage.",
			"The file input has a visible label that says the limits before anybody tries.",
			"A refused file comes back as a 422 with the message on the field and the focus on it.",
		},
	},
}
