package cookbook

import (
	"database/sql"
	"os"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/ui"
)

// The code behind "The management app in one afternoon". What
// `trilha new --template app` writes is not here — it is written by the
// recipes, and copying it would give the page a second source that goes
// stale. What is here is what the page asks you to write *after* the
// command: the module you add to the matrix, the middleware of a folder of
// your own, the menu the screens do not add themselves to, the query that
// carries the organisation, and the day the login stops being this app's.

// adminSessions is the session the login recipe writes into
// internal/sessao/sessao.go. It is here so the guards below have something
// to hang from; in your project it is the recipe's Flow.
var adminSessions = auth.Sessions(auth.Options{
	Store:      auth.NewMemoryStore(),
	Idle:       30 * time.Minute,
	LoginPath:  "/entrar",
	AfterLogin: "/",
})

// AdminPolicy is internal/acesso/acesso.go after your first module. Modules
// are what the application protects, Levels are ordered and each one implies
// the ones below, and a role is what a person carries in their session.
//
// Adding "pedidos" here is the whole change: the grid grows a column, the
// roles screen grows a cell, and the middleware below has something to ask
// for. Nothing else in the application knows the name of a role.
var AdminPolicy = auth.Policy{
	Modules: []string{"relatorios", "usuarios", "pedidos"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},
	Roles: map[string]auth.Grants{
		"admin":     auth.All("administrar"),
		"comercial": {"pedidos": "editar", "relatorios": "ver"},
		"leitor":    {"relatorios": "ver"},
	},
}

// adminOrders is the guard of app/admin/pedidos: the matrix answers, and the
// answer is a middleware. Somebody signed in without the level gets 403 and
// not the login screen — they are known, just not permitted.
var adminOrders = adminSessions.RequirePolicy(AdminPolicy, "pedidos", "editar")

// AdminOrdersMiddleware is what the scanner asks for: a function with a fixed
// signature. A var of the right type is not one, which is why the guard and
// the middleware are two names.
func AdminOrdersMiddleware(c *trilha.Ctx, next trilha.Next) error { return adminOrders(c, next) }

// AdminOwnRows is the rule a matrix cannot hold: a level is about a module,
// and "only in the organisation they are in" is about a row. RequireFunc is
// where that lives, deliberately — a policy that could express it would be a
// policy nobody could read on a screen.
var AdminOwnRows = adminSessions.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
	return u.Tenant != "" && u.Tenant == c.Param("org")
})

// AdminNav is the menu of app/layout.go with the screens the recipes wrote.
// They do not add themselves to it: a recipe that edited the layout would be
// a recipe rewriting a file you own.
//
// Hide is cosmetics. What keeps somebody out of /admin is the middleware.go
// of that branch, and hiding the link only saves them the 403.
func AdminNav(admin bool) []ui.NavGroup {
	return []ui.NavGroup{
		{Label: "Work", Items: []ui.NavItem{
			{Href: "/", Label: "Dashboard", Icon: "house"},
			{Href: "/admin/pedidos", Label: "Orders", Icon: "search"},
			{Href: "/perfil", Label: "My account", Icon: "user"},
			{Href: "/organizacoes", Label: "Organisations", Icon: "building"},
		}},
		{Label: "Admin", Hide: !admin, Items: []ui.NavItem{
			{Href: "/admin/usuarios", Label: "Users", Icon: "users"},
			{Href: "/admin/permissoes", Label: "Permissions", Icon: "lock"},
			{Href: "/admin/auditoria", Label: "Audit", Icon: "list"},
			{Href: "/admin/chaves", Label: "API keys", Icon: "key"},
			{Href: "/admin/config", Label: "Settings", Icon: "settings"},
		}},
	}
}

// AdminOrder is one row of your own table, with the column that decides who
// may see it.
type AdminOrder struct {
	ID      string
	Cliente string
	Total   int
}

// AdminOrdersOf is the query, and the filter that cannot be missing. The
// organisation comes from the session — never from the URL, never from a
// hidden field — because a value the browser sends is a value the browser
// chooses.
//
// auth.Tenant(c) is one call so the WHERE reads like a WHERE. Nothing in the
// framework writes this clause for you: a clause generated somewhere else is
// a clause nobody checks in review, which is the opposite of what a tenant
// filter needs.
func AdminOrdersOf(c *trilha.Ctx, db *sql.DB) ([]AdminOrder, error) {
	rows, err := db.QueryContext(c.Context(), `
		SELECT id, cliente, total
		  FROM pedidos
		 WHERE org_id = $1
		 ORDER BY criado_em DESC
		 LIMIT 50`, auth.Tenant(c))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminOrder
	for rows.Next() {
		var o AdminOrder
		if err := rows.Scan(&o.ID, &o.Cliente, &o.Total); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AdminTenantGuard goes below the session guard: a person signed in with no
// organisation chosen is not an intruder, it is somebody who has not picked
// one yet. A browser lands on the picker; anything else gets 403, because a
// redirect to a screen is not an answer an API can use.
var AdminTenantGuard = adminSessions.RequireTenant()

// AdminGeneral is a settings section: the struct is the screen. Labels come
// from the tags, controls from the types, validation from the same rules a
// form uses — and the defaults are what the application runs on before
// anybody opens the screen, so they have to be a working configuration.
type AdminGeneral struct {
	Nome    string `form:"nome" label:"Application name" validate:"required,max=60"`
	Suporte string `form:"suporte" label:"Support e-mail" validate:"required,email"`
	Dias    int    `form:"dias" label:"Days before expiry" validate:"min=1,max=365"`
}

// AdminSettings is the section itself, the one app/admin/config edits.
var AdminSettings = trilha.NewSettings("geral", AdminGeneral{
	Nome:    "My application",
	Suporte: "support@example.com",
	Dias:    30,
})

// AdminSSO is the day the login stops being this application's: an identity
// provider says who the person is, and the users table stops holding
// passwords.
//
// RequireVerifiedEmail belongs to that day and not to the one before it. It
// reads the provider's email_verified claim, so it has something to read only
// when there is a provider; with the local login the equivalent is the
// two-step change in /perfil, where the link goes to the new address and only
// the route it opens changes the row.
func AdminSSO() *auth.Auth {
	p := auth.OIDC(
		os.Getenv("OIDC_ISSUER"),
		os.Getenv("OIDC_CLIENT_ID"),
		os.Getenv("OIDC_CLIENT_SECRET"),
		"https://empresa.example.com/auth/callback",
	)
	return auth.New(p, auth.Options{
		Store:                auth.NewMemoryStore(),
		RequireVerifiedEmail: true,
		LoginPath:            "/entrar",
		AfterLogin:           "/",
	})
}
