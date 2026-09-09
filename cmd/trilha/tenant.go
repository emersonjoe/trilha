package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Forgetting the tenant column in one query out of forty is the most common
// bug of the most common shape of multi-tenant, and it is invisible in review:
// the query looks like every other query. It is not invisible to counting,
// though — a table that is filtered by tenant in seven places and not in the
// eighth is a very good guess.
//
// This is a text heuristic and says so. There is no SQL parser here, and there
// will not be one: the answer it gives is a place to look, not a verdict.

var (
	// reSQL finds the string literals that are queries. Both quoting styles,
	// because a multi-line query is a raw string and a one-liner usually is
	// not.
	reSQL = regexp.MustCompile("(?is)`([^`]*(?:select|insert|update|delete)[^`]*)`|\"([^\"\\n]*(?:SELECT|INSERT|UPDATE|DELETE)[^\"\\n]*)\"")
	// reTable finds what a query touches. It is deliberately narrow: a name
	// after FROM, JOIN, INTO or UPDATE, and nothing clever about subqueries.
	reTable = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+"?([a-z_][a-z0-9_]*)"?`)
	// reTenantCol is the column, in the spellings people actually use.
	reTenantCol = regexp.MustCompile(`(?i)\b(tenant_id|tenant|org_id|organization_id|organizacao_id)\b`)
)

// tenantQuery is one query found in the source.
type tenantQuery struct {
	file   string
	line   int
	tables []string
	tenant bool
}

// tenantGaps lists the queries that touch a table everybody else filters by
// tenant. The message names the table and the count, because "documents is
// filtered in 7 queries and not in this one" is a sentence somebody can act on
// and "possible multi-tenant issue" is not.
func tenantGaps(root string) []string {
	queries := collectQueries(root)
	if len(queries) == 0 {
		return nil
	}
	// How many queries filter each table by tenant, and how many touch it at
	// all. A table nobody filters is a table with no tenant column, and that
	// is not a finding.
	filtered, total := map[string]int{}, map[string]int{}
	for _, q := range queries {
		for _, tbl := range q.tables {
			total[tbl]++
			if q.tenant {
				filtered[tbl]++
			}
		}
	}
	var out []string
	for _, q := range queries {
		if q.tenant {
			continue
		}
		for _, tbl := range q.tables {
			// One filtered query is not a pattern; two is. The threshold is
			// low on purpose — this points, it does not block.
			if filtered[tbl] < 2 {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d: %s", q.file, q.line,
				fmt.Sprintf(t("tenant gap"), tbl, filtered[tbl], total[tbl])))
			break
		}
	}
	sort.Strings(out)
	return out
}

// collectQueries reads the project's own code, with positions — the concat of
// projectSource has no line numbers, and a warning without a line is a warning
// somebody has to go looking for.
func collectQueries(root string) []tenantQuery {
	var out []tenantQuery
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".trilha", "node_modules", "vendor", "bin", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(data)
		rel, _ := filepath.Rel(root, path)
		for _, m := range reSQL.FindAllStringSubmatchIndex(src, -1) {
			lit := src[m[0]:m[1]]
			tables := tablesOf(lit)
			if len(tables) == 0 {
				continue
			}
			out = append(out, tenantQuery{
				file:   filepath.ToSlash(rel),
				line:   1 + strings.Count(src[:m[0]], "\n"),
				tables: tables,
				tenant: reTenantCol.MatchString(lit),
			})
		}
		return nil
	})
	return out
}

func tablesOf(lit string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range reTable.FindAllStringSubmatch(lit, -1) {
		tbl := strings.ToLower(m[1])
		// The words that follow FROM or UPDATE without being a table.
		switch tbl {
		case "select", "set", "where", "values", "dual":
			continue
		}
		if !seen[tbl] {
			seen[tbl] = true
			out = append(out, tbl)
		}
	}
	return out
}
