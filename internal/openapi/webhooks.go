package openapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/scan"
)

// The headers every delivery carries. They are here and not in a comment
// because whoever integrates from the other side reads the document, not this
// file.
var deliveryHeaders = []struct{ name, desc string }{
	{"X-Webhook-Id", "Id of this delivery. The same id repeats on a retry, so the receiver can make the handling idempotent."},
	{"X-Webhook-Event", "Name of the event, the same key as in this section."},
	{"X-Webhook-Timestamp", "Unix seconds of the delivery, and the first half of what the signature covers."},
	{"X-Webhook-Signature", "sha256=… — HMAC-SHA256 of \"timestamp.body\" with the subscription's secret. webhook.Verify checks it."},
}

// webhooks is the other half of the contract: what the application sends.
//
// The events come from the calls to Emit, with the name written in the call
// and the body read from the third argument by the same machinery that reads
// the body of a route — so a struct that is already a component is referenced
// and not copied.
//
// The rule is mechanical, like the rest of this generator: an Emit whose event
// name comes from a variable is not documented, because a document cannot
// state a name that does not exist until the program runs.
func (g *generator) webhooks(root string) map[string]*pathItem {
	found := map[string]*schema{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p == root {
				return nil
			}
			if name != scan.WellKnown && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		g.emitsIn(root, p, found)
		return nil
	})
	if len(found) == 0 {
		return nil
	}
	out := make(map[string]*pathItem, len(found))
	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out[n] = &pathItem{Post: webhookOperation(n, found[n])}
	}
	return out
}

// emitsIn reads one file and collects the events it sends.
func (g *generator) emitsIn(root, file string, found map[string]*schema) {
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		return // the compiler is the one who complains about a broken file
	}
	rel, err := filepath.Rel(root, filepath.Dir(file))
	if err != nil {
		return
	}
	imp := g.ix.module
	if rel != "." {
		imp = path.Join(g.ix.module, filepath.ToSlash(rel))
	}
	sc := scopeOf(f, imp)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// The locals are tracked the same way a handler's are, because the
		// payload is usually a variable: it := store.Create(…); Hooks.Emit(c,
		// "item.created", it).
		locals := map[string]typeRef{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.DeclStmt:
				// var in struct{…} is how a handler declares the body it is
				// about to fill, and it is the shape an Emit usually carries.
				gd, ok := n.Decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					return true
				}
				for _, sp := range gd.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok || vs.Type == nil {
						continue
					}
					for _, id := range vs.Names {
						locals[id.Name] = typeRef{expr: vs.Type, scope: sc}
					}
				}
			case *ast.AssignStmt:
				g.assign(n, sc, locals)
			case *ast.CallExpr:
				name, payload, ok := emitCall(n)
				if !ok {
					return true
				}
				if _, seen := found[name]; seen {
					// The first one wins, and the second is not an error: the
					// same event sent from two places is one event.
					return true
				}
				found[name] = g.valueSchema(payload, sc, locals)
			}
			return true
		})
	}
}

// emitCall recognises hooks.Emit(ctx, "event.name", payload).
func emitCall(call *ast.CallExpr) (string, ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Emit" || len(call.Args) < 3 {
		return "", nil, false
	}
	name, ok := stringOf(call.Args[1])
	if !ok || name == "" {
		return "", nil, false
	}
	return name, call.Args[2], true
}

// webhookOperation is one event as OpenAPI 3.1 describes it: a request the
// application makes to whoever subscribed.
func webhookOperation(name string, body *schema) *operation {
	op := &operation{
		Summary:     "Sent when " + name + " happens.",
		OperationID: "webhook_" + strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(name),
		Responses: map[string]*response{
			"200": {Description: "Received. Any 2xx within the timeout ends the delivery; anything else is retried with backoff."},
		},
	}
	for _, hd := range deliveryHeaders {
		op.Parameters = append(op.Parameters, &parameter{
			Name: hd.name, In: "header", Required: true, Description: hd.desc,
			Schema: &schema{Type: "string"},
		})
	}
	if body != nil {
		op.RequestBody = &requestBody{Required: true, Content: map[string]mediaType{jsonMediaType: {Schema: body}}}
	}
	return op
}
