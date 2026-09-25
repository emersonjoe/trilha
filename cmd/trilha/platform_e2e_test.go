package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// platformTests are the tests the platform recipes write into the project
// (spec 162), by package: named here so that deleting one is a failure and not
// a quieter test run.
var platformTests = map[string][]string{
	".": {
		"TestBillingWebhookValid", "TestBillingWebhookUnsigned", "TestBillingWebhookExpired",
		"TestBillingIdempotentEvent", "TestBillingDunningCycle", "TestBillingCSVAdminOnly",
		"TestBillingScreensKeepProtections", "TestNotifyPreferences", "TestAdminDefaultDeny",
	},
	"./internal/cobranca/":  {"TestBillingStates"},
	"./internal/notificar/": {"TestNotifyQuietHours", "TestNotifyDigest", "TestNotifyRateLimit"},
}

func init() {
	platformTests["."] = append(platformTests["."], "TestNotifyPreferences", "TestNotifyOutboxReplay", "TestNotifyChannelsLinked",
		"TestAdminDefaultDeny", "TestAdminAuditTrail", "TestAdminApprovalFlow")
}

// notifyLinksTest is written by the e2e itself, the way chavesRevogarTest is:
// with webhooks and channel-whatsapp in the project, notify offers all three
// channels, the webhook event is in the closed list the deliverer reads, and
// a notification by webhook leaves.
const notifyLinksTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"

	"example.com/plataforma/internal/notificar"
)

func TestNotifyChannelsLinked(t *testing.T) {
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	if got := notificar.Disponiveis(); !slices.Equal(got, []string{"mail", "webhook", "whatsapp"}) {
		t.Fatalf("channels = %v", got)
	}
	if !slices.Contains(trilha.Use[*webhook.Hooks](a).Events(), notificar.Evento) {
		t.Fatalf("%s is not in the deliverer's list: %v", notificar.Evento, trilha.Use[*webhook.Hooks](a).Events())
	}
	trilha.Use[*notificar.Notificador](a).SalvarPreferencias("u-9", notificar.Preferencias{Canal: notificar.CanalWebhook})
	a.Register(trilha.Route{Pattern: "/_teste/webhook", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				no, err := trilha.Use[*notificar.Notificador](c).Notificar(c,
					notificar.Pessoa{Sujeito: "u-9", Email: "nove@example.com"}, "t", "c")
				if err != nil {
					return err
				}
				return c.Text(http.StatusOK, string(no.Estado)+" "+no.Canal)
			},
		}})
	trilha.NewTestClient(t, a).PostForm("/_teste/webhook", url.Values{}).
		WantStatus(http.StatusOK).WantContains("enviada webhook")
}
`

// TestAddPlatformE2E is spec 162 end to end: the platform recipes applied to a
// project nobody touched, next to the recipes they tie themselves to, with the
// gate green and every test the plan names passing — no edit in between.
func TestAddPlatformE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	t.Setenv("TRILHA_LANG", "en")
	t.Setenv("TRILHA_SECRET", "um-segredo-de-teste-com-mais-de-32-bytes")
	cli := buildCLI(t, repo, tmp)

	proj := filepath.Join(tmp, "plataforma")
	run(t, tmp, cli, "new", proj, "--module", "example.com/plataforma", "--trilha-dir", repo)

	// Billing refuses before it writes anything when what it stands on is
	// missing, and says what to run.
	if out, err := runErr(t, proj, cli, "add", "billing"); err == nil {
		t.Fatalf("add billing wrote without login:\n%s", out)
	} else if !strings.Contains(out, "trilha add login") {
		t.Fatalf("the refusal does not say what to run first:\n%s", out)
	}

	out := run(t, proj, cli, "add", "login", "connections", "notify", "webhooks", "channel-whatsapp", "billing", "admin")
	// The add ends with the price of reading what arrived — measured in this
	// project (spec 165), so it is the number `ctx --pack` will cost here.
	var measured struct {
		Used int `json:"used_tokens"`
	}
	if err := json.Unmarshal([]byte(run(t, proj, cli, "ctx", "--pack", "billing", "--json")), &measured); err != nil || measured.Used == 0 {
		t.Fatalf("ctx --pack billing --json: %v %+v", err, measured)
	}
	if want := fmt.Sprintf("`trilha ctx --pack billing` costs ~%d tokens (est.)", measured.Used); !strings.Contains(out, want) {
		t.Fatalf("add did not end with the price measured here %q:\n%s", want, out)
	}

	mustWrite(t, filepath.Join(proj, "notify_links_test.go"), notifyLinksTest)

	// The gate, which runs every test the recipes wrote — and the audit, with
	// the rule billing brought.
	if out := run(t, proj, cli, "check"); !strings.Contains(out, "test") {
		t.Fatal(out)
	}
	audit := run(t, proj, cli, "audit", "--no-vuln")
	if !strings.Contains(audit, "billing: the provider's secret lives in connections kept in memory") {
		t.Fatalf("the audit does not say where billing's secret lives:\n%s", audit)
	}
	for pkg, names := range platformTests {
		testOut := run(t, proj, "go", "test", pkg, "-run", "^("+strings.Join(names, "|")+")$", "-v")
		for _, name := range names {
			if !strings.Contains(testOut, "--- PASS: "+name) {
				t.Errorf("%s did not pass in %s:\n%s", name, pkg, testOut)
			}
		}
	}

	// The pack the price is about answers in a real project: the recipe's
	// files and the fixed address the provider calls.
	pack := run(t, proj, cli, "ctx", "--pack", "billing", "--json")
	for _, want := range []string{`"name": "billing"`, `"/webhooks/billing"`, `"internal/cobranca/webhook.go"`} {
		if !strings.Contains(pack, want) {
			t.Errorf("ctx --pack billing does not carry %s:\n%s", want, pack)
		}
	}

	// A second run writes nothing.
	if again := run(t, proj, cli, "add", "billing"); strings.Contains(again, "  + ") {
		t.Fatalf("running it again wrote a file:\n%s", again)
	}
	if _, err := os.Stat(filepath.Join(proj, "migrations", "0100_billing.sql")); err != nil {
		t.Fatal("the migration is not there")
	}
}
