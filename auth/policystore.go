package auth

import (
	"context"
	"strings"

	"github.com/emersonjoe/trilha"
)

// PolicyStore is where an application keeps a matrix that people edit. The
// modules and the levels stay in the code — they are the shape of the
// application and change when the code does — and only the roles come from
// here, because a role is what an administrator invents on a Tuesday.
//
// Two methods and not three: Load already returns the roles, so a separate
// Roles would be the same answer with a second way to be stale.
type PolicyStore interface {
	Load(ctx context.Context) (map[string]Grants, error)
	Save(ctx context.Context, roles map[string]Grants) error
}

// PolicyFrom returns the policy with the roles the store holds. An empty store
// keeps the roles already declared, which is what makes the first boot of a new
// installation work without a migration.
//
// It is a snapshot on purpose. A Policy is a value, and a value that changed
// underneath a request would let one request answer twice — allowed at the
// middleware, denied at the button. Call it again after Save; the example does
// exactly that, in one line.
func PolicyFrom(ctx context.Context, s PolicyStore, defaults Policy) (Policy, error) {
	if s == nil {
		return defaults, nil
	}
	roles, err := s.Load(ctx)
	if err != nil {
		return defaults, err
	}
	if len(roles) == 0 {
		return defaults, nil
	}
	out := defaults
	out.Roles = roles
	return out, nil
}

// BindPolicy reads what the grid posted: one field per cell, named
// "grant.<role>.<module>", holding a level or the empty string for none.
//
// A cell naming a module or a level the policy does not declare is dropped, not
// refused: the form is the only way in, and the answer to a field that should
// not exist is to not write it down — a 400 here would only tell whoever forged
// it which name to try next. A role with no cell at all disappears, which is
// how the grid deletes one.
func BindPolicy(c *trilha.Ctx, p Policy) (map[string]Grants, error) {
	if err := c.Request().ParseForm(); err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, m := range p.Modules {
		known[m] = true
	}
	out := map[string]Grants{}
	for field, values := range c.Request().PostForm {
		rest, ok := strings.CutPrefix(field, "grant.")
		if !ok || len(values) == 0 {
			continue
		}
		role, module, ok := strings.Cut(rest, ".")
		if !ok || role == "" || !known[module] {
			continue
		}
		if _, seen := out[role]; !seen {
			out[role] = Grants{}
		}
		level := strings.TrimSpace(values[0])
		if level == "" {
			continue // the cell is there and says nothing: no access
		}
		if p.Levels.rank(level) < 0 {
			continue
		}
		out[role][module] = level
	}
	return out, nil
}
