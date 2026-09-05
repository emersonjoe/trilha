package auth

// roles extracts the role names from the claims, in the place each provider
// puts them. Nothing here is guessed at runtime: the provider was chosen by
// the app when it called EntraID, Keycloak, Cognito or OIDC.
func (p *Provider) roles(c *Claims, extra []string) []string {
	var out []string
	add := func(v any) {
		switch t := v.(type) {
		case string:
			out = append(out, t)
		case []any:
			for _, e := range t {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	switch p.kind {
	case entraProvider:
		// App roles assigned in the enterprise application, plus group ids
		// (Entra sends object ids unless the app maps names).
		add(c.All["roles"])
		add(c.All["groups"])
		add(c.All["wids"])
	case keycloakProvider:
		if ra, ok := c.All["realm_access"].(map[string]any); ok {
			add(ra["roles"])
		}
		if res, ok := c.All["resource_access"].(map[string]any); ok {
			if cl, ok := res[p.keycloakClient].(map[string]any); ok {
				add(cl["roles"])
			}
		}
	case cognitoProvider:
		// Cognito puts the user pool groups here; there is no separate roles claim.
		add(c.All["cognito:groups"])
	default:
		add(c.All["roles"])
		add(c.All["groups"])
	}
	for _, claim := range extra {
		add(c.All[claim])
	}
	return dedup(out)
}

func dedup(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
