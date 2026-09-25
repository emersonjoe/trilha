package sqltest

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// postgres starts a PostgreSQL cluster of this test's own — initdb in a
// temporary directory, trust authentication, listening only on 127.0.0.1 at
// a free port — and answers the banco that makes a database per test in it.
// Nothing outside the temporary directory is touched, and the cluster stops
// when the test ends.
func postgres(t *testing.T) banco {
	t.Helper()
	initdb, pgctl, psql := pgProgram("initdb"), pgProgram("pg_ctl"), pgProgram("psql")
	if initdb == "" || pgctl == "" || psql == "" {
		if os.Getenv("SQLTEST_POSTGRES") == "1" {
			t.Fatal("PostgreSQL's initdb, pg_ctl and psql are not in PATH, and SQLTEST_POSTGRES=1")
		}
		t.Skip("PostgreSQL's initdb, pg_ctl and psql are not in PATH")
	}
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	must(t, sh(dir, nil, initdb, "-D", data, "-U", "postgres", "--auth=trust", "-E", "UTF8"))
	port := freePort(t)
	// TCP on 127.0.0.1 only, and no Unix socket: the default directory is
	// somebody else's, and a temporary one is past the 103 bytes macOS allows
	// a socket path.
	opts := fmt.Sprintf("-p %s -c listen_addresses=127.0.0.1 -c unix_socket_directories=''", port)
	if err := sh(dir, nil, pgctl, "-D", data, "-o", opts, "-l", filepath.Join(dir, "log"), "-w", "start"); err != nil {
		log, _ := os.ReadFile(filepath.Join(dir, "log"))
		t.Fatalf("%v\npostgres log:\n%s", err, log)
	}
	t.Cleanup(func() { _ = sh(dir, nil, pgctl, "-D", data, "-m", "fast", "-w", "stop") })

	psqlc := func(t *testing.T, db, sql string) string {
		t.Helper()
		out, err := output(dir, nil, psql, "-h", "127.0.0.1", "-p", port, "-U", "postgres", "-d", db, "-tAc", sql)
		if err != nil {
			t.Fatalf("psql %q: %v\n%s", sql, err, out)
		}
		return strings.TrimSpace(out)
	}
	ident := regexp.MustCompile(`[^a-z0-9_]`)
	n := 0
	return banco{
		nome:   "PostgreSQL",
		driver: "github.com/jackc/pgx/v5/stdlib",
		get:    "github.com/jackc/pgx/v5@v5.7.2",
		novo: func(t *testing.T, teste string) string {
			n++
			name := "t" + strconv.Itoa(n) + "_" + ident.ReplaceAllString(strings.ToLower(teste), "_")
			if len(name) > 60 {
				name = name[:60]
			}
			psqlc(t, "postgres", "CREATE DATABASE "+name)
			return "postgres://postgres@127.0.0.1:" + port + "/" + name + "?sslmode=disable"
		},
		linhas: func(t *testing.T, url, table string) int {
			db := url[strings.LastIndex(url, "/")+1 : strings.Index(url, "?")]
			k, err := strconv.Atoi(psqlc(t, db, "SELECT count(*) FROM "+table))
			must(t, err)
			return k
		},
	}
}

// pgProgram finds one of PostgreSQL's programs: in PATH, where Homebrew keeps
// a versioned install, or where Debian and Ubuntu do.
func pgProgram(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	matches, _ := filepath.Glob("/opt/homebrew/opt/postgresql@*/bin/" + name)
	if len(matches) == 0 {
		matches, _ = filepath.Glob("/usr/lib/postgresql/*/bin/" + name) // Debian and Ubuntu
	}
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	return ""
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	must(t, err)
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	must(t, err)
	return port
}
