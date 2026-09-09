package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// A hierarchy with thousands of nodes is the component people go to npm for:
// expanding, searching and the keyboard are each easy and together are three
// hundred lines. The trick here is that the server already knows the tree, so
// the browser never has to: a node is <details>, its children are a fragment,
// and the script only saves the round trip of a page.

// TreeNode is one node. Children that are already known travel with it — the
// open path when the screen loads, or a small tree served whole.
type TreeNode struct {
	// Value is what the form posts and what the source is asked about.
	Value string
	// Label is what a person reads.
	Label string
	// Leaf says there is nothing under it, so it draws no arrow and is never
	// asked for children.
	Leaf bool
	// Href turns the label into a link — for a node that is also a page. The
	// picker ignores it: there, clicking a node chooses it.
	Href string
	// Children are already loaded; nil means "ask the source when opened".
	Children []TreeNode
	// Open draws it expanded, which is how the path down to the current node
	// arrives open on the first render.
	Open bool
	// Path is the ancestry shown under a search result — "100 › 100.1" — so a
	// match out of context still says where it lives.
	Path string
}

// TreeOpts is the tree itself.
type TreeOpts struct {
	// Nodes are the roots, with whatever children came with them.
	Nodes []TreeNode
	// Source is a GET route that answers TreeNodes for ?parent=<value>. Empty
	// means the whole tree is in Nodes and nothing is ever fetched.
	Source string
	// Current is the value marked as the current node.
	Current string
	// Label is the accessible name of the tree — a tree announced as "tree"
	// and nothing else is a tree nobody can tell from the next one.
	Label string
	// ID is the element's id; one is generated from Label when empty.
	ID string
	// Name turns every node into a radio of this name: the tree stops being a
	// navigation and becomes a field. TreePicker sets it.
	Name string
	// Selected is the value already chosen, when Name is set.
	Selected string
}

// Tree renders a hierarchy that opens node by node.
//
//	ui.Tree(ui.TreeOpts{
//		Nodes:   roots,                 // with the open path already inside
//		Source:  "/classification/nodes", // GET ?parent=100.1 answers the children
//		Current: doc.Code,
//		Label:   "Classification plan",
//	})
//
//	// app/classification/nodes/route.go
//	func GET(c *trilha.Ctx) error {
//		return c.HTML(200, ui.TreeNodes(children(c.Query("parent")), func(n Node) ui.TreeNode {
//			return ui.TreeNode{Value: n.Code, Label: n.Code + " " + n.Name, Leaf: n.Leaves == 0}
//		}))
//	}
//
// Each node is a <details>, so it opens with no JavaScript at all; what the
// script adds is fetching the children the first time instead of asking the
// server for a whole page. A node whose children are already in Nodes never
// asks for anything, which is why the path down to Current arrives open and
// complete on the first render.
//
// The roles are the real ones — tree, treeitem, group, aria-expanded — and the
// arrows, Home, End and "*" move through it. Load TreeScript once on the page.
//
//	see: ui.TreePicker, ui.TreeNodes, ui.Combobox
func Tree(o TreeOpts) h.Node {
	id := o.ID
	if id == "" {
		id = "ui-tree"
	}
	// One role, chosen here: a tree of radios is a field and is announced as a
	// group of choices; a tree of links is a navigation. Emitting both would
	// be two role attributes on one element, which is not a stricter promise —
	// it is an invalid one.
	role := "tree"
	if o.Name != "" {
		role = "group"
	}
	kids := []h.Node{
		h.ID(id), h.Class("ui-tree"), h.Role(role), h.Data("ui-tree", ""),
	}
	if o.Source != "" {
		kids = append(kids, h.Data("ui-tree-src", o.Source))
	}
	if o.Label != "" {
		kids = append(kids, h.Aria("label", o.Label))
	}
	for _, n := range o.Nodes {
		kids = append(kids, treeItem(n, o))
	}
	return h.Div(kids...)
}

// TreeNodes is what a source route answers: the children of one node, as HTML,
// because the browser has nothing to decide about them.
//
//	return c.HTML(200, ui.TreeNodes(children, func(n Node) ui.TreeNode { … }))
func TreeNodes[T any](items []T, of func(T) TreeNode) h.Node {
	nodes := make([]TreeNode, 0, len(items))
	for _, it := range items {
		nodes = append(nodes, of(it))
	}
	return TreeItems(nodes, TreeOpts{})
}

// TreeItems renders nodes with the options of the tree they belong to — the
// name of the radios, the current value — for a source route answering inside
// a picker, where the children have to come back as fields and not as text.
//
//	return c.HTML(200, ui.TreeItems(nodes, ui.TreeOpts{Name: "code"}))
func TreeItems(nodes []TreeNode, o TreeOpts) h.Node {
	items := make([]h.Node, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, treeItem(n, o))
	}
	return h.Group(items...)
}

// treeItem is one node: a leaf is a single line, and a branch is a <details>
// carrying its children or the promise of them.
func treeItem(n TreeNode, o TreeOpts) h.Node {
	label := treeLabel(n, o)
	if n.Leaf {
		return h.Div(h.Class("ui-tree-item"), h.Role("treeitem"),
			h.Data("value", n.Value), h.Tabindex("-1"), label)
	}
	det := []h.Node{
		h.Class("ui-tree-item ui-tree-branch"), h.Role("treeitem"),
		h.Data("value", n.Value), h.Tabindex("-1"),
		h.Aria("expanded", boolAttr(n.Open)),
	}
	if n.Open {
		det = append(det, h.Open())
	}
	det = append(det, h.Summary(h.Class("ui-tree-summary"), label))

	group := []h.Node{h.Class("ui-tree-group"), h.Role("group")}
	for _, c := range n.Children {
		group = append(group, treeItem(c, o))
	}
	if len(n.Children) == 0 {
		// Nothing to show yet: the script fills this in on the first open, and
		// with no script it stays as the sentence rather than as a silence
		// that reads like an empty branch.
		group = append(group, h.Div(h.Class("ui-tree-pending"), h.Data("ui-tree-pending", "")))
	}
	det = append(det, h.Div(group...))
	return h.Details(det...)
}

// treeLabel is the node itself: a radio inside a picker, a link when the node
// is also a page, and plain text otherwise.
func treeLabel(n TreeNode, o TreeOpts) h.Node {
	text := []h.Node{h.Text(n.Label)}
	if n.Path != "" {
		text = append(text, h.Span(h.Class("ui-tree-path"), h.Text(n.Path)))
	}
	switch {
	case o.Name != "":
		attrs := []h.Node{h.Type("radio"), h.Name(o.Name), h.Value(n.Value), h.Class("ui-tree-radio")}
		if n.Value == o.Selected {
			attrs = append(attrs, h.Checked())
		}
		return h.Label(h.Class("ui-tree-label"), h.Input(attrs...), h.Span(text...))
	case n.Href != "":
		kids := append([]h.Node{h.Class("ui-tree-label"), h.Href(n.Href)}, text...)
		if n.Value == o.Current {
			kids = append(kids, h.Aria("current", "true"))
		}
		return h.A(kids...)
	}
	kids := []h.Node{h.Class("ui-tree-label")}
	if n.Value == o.Current {
		kids = append(kids, h.Aria("current", "true"))
	}
	return h.Span(append(kids, text...)...)
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TreePickerOpts is the field: a search that finds, a tree that browses, and
// one value that posts.
type TreePickerOpts struct {
	// Name is what the form posts — a radio per node, so choosing is a form
	// control and not a script.
	Name string
	// Value is what is already chosen.
	Value string
	// Nodes are the roots (with the path down to Value already open).
	Nodes []TreeNode
	// Source answers the children of ?parent=<value>.
	Source string
	// Search answers TreeNodes for ?q=, flattened, each with its Path — the
	// match of a code deep in the tree only means something with its ancestry
	// beside it. Empty draws no search field.
	Search string
	// Placeholder and Label are the search field's text and the tree's
	// accessible name.
	Placeholder string
	Label       string
	// MinChars is how much has to be typed before searching (2 when zero).
	MinChars int
}

// TreePicker is choosing one node out of thousands: type to find it, or browse
// to it.
//
//	ui.Field("code", "Classification", ui.TreePicker(ui.TreePickerOpts{
//		Name:   "code",
//		Value:  form.Code,
//		Nodes:  plan.Roots(form.Code), // open down to what is chosen
//		Source: "/classification/nodes",
//		Search: "/classification/search",
//	}))
//
// The value posts as a radio, which is the whole no-JavaScript story: a person
// with no script browses the same <details> and picks the same radio, and the
// form posts the same field. The search box then behaves like any filter — it
// submits, and the route renders the picker with the matches.
//
// With the script, typing asks Search and puts the matches in place of the
// tree, each with the ancestry it came from; clearing the box brings the tree
// back.
//
//	see: ui.Tree, ui.Combobox
func TreePicker(o TreePickerOpts, attrs ...h.Node) h.Node {
	min := o.MinChars
	if min < 1 {
		min = 2
	}
	tree := Tree(TreeOpts{
		Nodes:    o.Nodes,
		Source:   o.Source,
		Label:    o.Label,
		ID:       o.Name + "-tree",
		Name:     o.Name,
		Selected: o.Value,
	})
	box := []h.Node{
		h.Class("ui-tree-picker"), h.Data("ui-tree-picker", ""),
		h.Data("ui-tree-min", strconv.Itoa(min)),
	}
	if o.Search != "" {
		box = append(box, h.Data("ui-tree-search", o.Search))
		field := []h.Node{
			h.Type("search"), h.Name(o.Name + "_q"), h.Class("ui-input"),
			h.Attr("autocomplete", "off"),
		}
		if o.Label != "" {
			// An empty aria-label is worse than none: it names the field with
			// nothing and hides whatever the <label> around it would have said.
			field = append(field, h.Aria("label", o.Label))
		}
		if o.Placeholder != "" {
			field = append(field, h.Placeholder(o.Placeholder))
		}
		field = append(field, attrs...)
		box = append(box, h.Input(field...))
	}
	box = append(box, tree)
	return h.Div(box...)
}

// TreeScript loads ui.tree.js, the behaviour behind Tree and TreePicker: the
// children fetched on the first open, the search, and the keyboard. Put it
// once on the page that has a tree — Head does not load it, so a page without
// one does not download it.
func TreeScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.tree.js")), h.Defer())
}
