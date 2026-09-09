package recipes

// words is the text the recipes write into the project, per language.
//
// It is a table and not a translation mechanism, which is the same trade the
// rest of the scaffold makes: what comes out is a file somebody owns and
// edits, so a third language is a find-and-replace in their own code rather
// than a feature of ours.
var words = func(lang string) map[string]string {
	if lang == "pt" {
		return map[string]string{
			"audit_title": "Auditoria",
			"audit_desc":  "Quem fez o quê, em quê, e de onde.",

			"settings_title":        "Configurações",
			"settings_saved":        "Configurações salvas.",
			"settings_name":         "Nome da aplicação",
			"settings_support":      "E-mail de suporte",
			"settings_days":         "Dias até expirar",
			"settings_default_name": "Minha aplicação",

			"keys_title":     "Chaves de API",
			"keys_desc":      "A chave aparece uma vez, quando é criada. O que fica guardado é o hash dela.",
			"keys_name":      "Nome",
			"keys_scopes":    "Escopos",
			"keys_create":    "Criar chave",
			"keys_created":   "Chave criada. Copie agora.",
			"keys_revoked":   "Chave revogada.",
			"keys_need_name": "dê um nome à chave: é por ele que alguém vai saber qual revogar",
		}
	}
	return map[string]string{
		"audit_title": "Audit trail",
		"audit_desc":  "Who did what, to what, and from where.",

		"settings_title":        "Settings",
		"settings_saved":        "Settings saved.",
		"settings_name":         "Application name",
		"settings_support":      "Support e-mail",
		"settings_days":         "Days before expiry",
		"settings_default_name": "My application",

		"keys_title":     "API keys",
		"keys_desc":      "A key is shown once, when it is created. What is stored is its hash.",
		"keys_name":      "Name",
		"keys_scopes":    "Scopes",
		"keys_create":    "Create key",
		"keys_created":   "Key created. Copy it now.",
		"keys_revoked":   "Key revoked.",
		"keys_need_name": "give the key a name: it is how somebody later knows which one to revoke",
	}
}
