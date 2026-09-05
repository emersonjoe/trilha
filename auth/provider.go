package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Provider is an OpenID Connect provider. Everything but the identifiers
// comes from discovery, so a new provider needs no code here.
type Provider struct {
	// Issuer is the exact iss of the tokens, e.g.
	// https://login.microsoftonline.com/<tenant>/v2.0.
	Issuer string
	// ClientID and ClientSecret identify the application at the provider.
	ClientID, ClientSecret string
	// RedirectURL is the callback registered at the provider.
	RedirectURL string
	// HTTPClient talks to the provider (default: 10s timeout).
	HTTPClient *http.Client
	// LogoutDomain is the Cognito managed login domain
	// ("<prefix>.auth.<region>.amazoncognito.com" or your own), the only place
	// Cognito ends a session. Empty means local logout only. Ignored by every
	// other provider, which announces end_session_endpoint in discovery.
	LogoutDomain string
	// kind selects where roles live (see roles.go).
	kind providerKind
	// keycloakClient is the client id whose resource_access roles are read.
	keycloakClient string

	mu    sync.Mutex
	doc   *discovery
	docAt time.Time
	keys  *jwks
}

type providerKind int

const (
	genericProvider providerKind = iota
	entraProvider
	keycloakProvider
	cognitoProvider
)

// discovery is the subset of the provider metadata we use.
type discovery struct {
	Issuer        string `json:"issuer"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	JWKS          string `json:"jwks_uri"`
	EndSession    string `json:"end_session_endpoint"`
	UserInfo      string `json:"userinfo_endpoint"`
}

// OIDC configures any conforming provider by its issuer.
func OIDC(issuer, clientID, clientSecret, redirectURL string) *Provider {
	return &Provider{Issuer: strings.TrimSuffix(issuer, "/"), ClientID: clientID,
		ClientSecret: clientSecret, RedirectURL: redirectURL}
}

// EntraID configures Microsoft Entra ID (formerly Azure AD). tenant is the
// directory id, or "organizations"/"common" for multi-tenant. Roles come from
// the roles claim (app roles) and groups.
func EntraID(tenant, clientID, clientSecret, redirectURL string) *Provider {
	p := OIDC("https://login.microsoftonline.com/"+tenant+"/v2.0", clientID, clientSecret, redirectURL)
	p.kind = entraProvider
	return p
}

// Keycloak configures a Keycloak realm: baseURL is the server root
// (https://kc.exemplo.com), realm the realm name. Roles come from
// realm_access.roles and from resource_access[clientID].roles.
func Keycloak(baseURL, realm, clientID, clientSecret, redirectURL string) *Provider {
	p := OIDC(strings.TrimSuffix(baseURL, "/")+"/realms/"+realm, clientID, clientSecret, redirectURL)
	p.kind = keycloakProvider
	p.keycloakClient = clientID
	return p
}

// Cognito configures an Amazon Cognito user pool: region is the AWS region
// ("us-east-1") and userPoolID the pool id ("us-east-1_ABC123"). Roles come
// from cognito:groups. Cognito publishes no end_session_endpoint, so federated
// logout needs LogoutDomain; without it Logout clears the local session only.
func Cognito(region, userPoolID, clientID, clientSecret, redirectURL string) *Provider {
	p := OIDC("https://cognito-idp."+region+".amazonaws.com/"+userPoolID, clientID, clientSecret, redirectURL)
	p.kind = cognitoProvider
	return p
}

// endSession builds the URL that ends the session at the provider, plus the
// reason to log when there is none but there should be. Both empty is the
// ordinary case: the provider has no logout endpoint and clearing the local
// session is the whole story.
func (p *Provider) endSession(doc *discovery, postLogout string) (string, string) {
	if p.kind == cognitoProvider {
		// Not RP-Initiated Logout: Cognito ends the session with GET /logout on
		// the managed login domain, and logout_uri must be listed in the app
		// client's allowed sign-out URLs.
		if p.LogoutDomain == "" {
			return "", "Cognito publishes no end_session_endpoint; set Provider.LogoutDomain to end the session at the provider too"
		}
		host := strings.TrimSuffix(p.LogoutDomain, "/")
		if !strings.HasPrefix(host, "https://") && !strings.HasPrefix(host, "http://") {
			host = "https://" + host
		}
		q := url.Values{"client_id": {p.ClientID}, "logout_uri": {postLogout}}
		return host + "/logout?" + q.Encode(), ""
	}
	if doc.EndSession == "" {
		return "", ""
	}
	q := url.Values{"client_id": {p.ClientID}, "post_logout_redirect_uri": {postLogout}}
	return doc.EndSession + sep(doc.EndSession) + q.Encode(), ""
}

func (p *Provider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// discover fetches (and caches for an hour) the provider metadata.
func (p *Provider) discover(ctx context.Context) (*discovery, error) {
	p.mu.Lock()
	if p.doc != nil && time.Since(p.docAt) < time.Hour {
		doc := p.doc
		p.mu.Unlock()
		return doc, nil
	}
	p.mu.Unlock()

	url := p.Issuer + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var doc discovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("OIDC discovery unreadable: %w", err)
	}
	if doc.Issuer != p.Issuer {
		// The issuer in the document is the one that will be in the tokens;
		// a mismatch means we are talking to the wrong tenant or realm.
		return nil, fmt.Errorf("issuer mismatch: configured %q, document %q", p.Issuer, doc.Issuer)
	}
	if doc.Authorization == "" || doc.Token == "" || doc.JWKS == "" {
		return nil, fmt.Errorf("incomplete OIDC discovery at %s", url)
	}
	p.mu.Lock()
	p.doc, p.docAt = &doc, time.Now()
	if p.keys == nil || p.keys.url != doc.JWKS {
		p.keys = newJWKS(doc.JWKS, p.client())
	}
	p.mu.Unlock()
	return &doc, nil
}

// verify validates an ID token against the provider keys and claims.
func (p *Provider) verify(ctx context.Context, token, nonce string) (*Claims, error) {
	if _, err := p.discover(ctx); err != nil {
		return nil, err
	}
	hdr, payload, sig, signed, err := parseJWS(token)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	keys := p.keys
	p.mu.Unlock()
	key, err := keys.key(ctx, hdr.Kid)
	if err != nil {
		return nil, err
	}
	if err := verifySignature(hdr.Alg, key, signed, sig); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}
	claims, err := decodeClaims(payload)
	if err != nil {
		return nil, err
	}
	if err := claims.validate(p.Issuer, p.ClientID, nonce, time.Now()); err != nil {
		return nil, err
	}
	return claims, nil
}
