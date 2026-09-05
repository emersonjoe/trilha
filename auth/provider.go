package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return nil, fmt.Errorf("descoberta OIDC falhou: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("descoberta OIDC respondeu %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var doc discovery
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("descoberta OIDC ilegível: %w", err)
	}
	if doc.Issuer != p.Issuer {
		// The issuer in the document is the one that will be in the tokens;
		// a mismatch means we are talking to the wrong tenant or realm.
		return nil, fmt.Errorf("emissor divergente: configurado %q, documento %q", p.Issuer, doc.Issuer)
	}
	if doc.Authorization == "" || doc.Token == "" || doc.JWKS == "" {
		return nil, fmt.Errorf("descoberta OIDC incompleta em %s", url)
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
		return nil, fmt.Errorf("assinatura inválida: %w", err)
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
