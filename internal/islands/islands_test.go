package islands

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden file")

const (
	appDir     = "../../testdata/islands/app"
	goldenPath = "../../testdata/islands/islands.d.ts"
)

func scan(t *testing.T) *Result {
	t.Helper()
	r, err := Scan(appDir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestGolden(t *testing.T) {
	got := scan(t).TypeScript()
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("islands.d.ts changed; run make golden\n--- got ---\n%s", got)
	}
}

// The same app read twice gives the same file: the generated file is committed,
// so a map walked in a different order would show up as a diff.
func TestDeterministic(t *testing.T) {
	first := scan(t).TypeScript()
	for i := 0; i < 5; i++ {
		if got := scan(t).TypeScript(); got != first {
			t.Fatal("two reads of the same app disagree")
		}
	}
}

// The props of an island are read from the struct, with the names encoding/json
// uses and the types JSON.parse gives back.
func TestPropsComeFromTheStruct(t *testing.T) {
	ts := scan(t).TypeScript()
	for _, want := range []string{
		`"/editor.js": EditorProps;`,
		"wpm: number;",                    // int
		"status: string;",                 // a defined string type
		"tags: string[];",                 // slice
		"draft?: Revision;",               // pointer plus omitempty, and a nested interface
		"counts: Record<string, number>;", // map
		"raw: string;",                    // []byte is base64
		"size: { rows: number };",         // anonymous struct
		"name: string;",                   // embedded, flattened the way encoding/json does
		"email?: string;",                 // omitempty
		"at: string;",                     // time.Time
		"prev?: Revision;",                // a struct that points at itself
		"/** WPM is the reading speed shown under the counter. */",
		"/** EditorProps is everything the editor needs to take over the textarea. */",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("missing %q in:\n%s", want, ts)
		}
	}
	for _, gone := range []string{"Secret", "internal"} {
		if strings.Contains(ts, gone) {
			t.Errorf("%q is not in the JSON, so it is not in the types", gone)
		}
	}
}

// What the reader cannot name it says out loud, and the island still exists:
// props typed unknown is a missing type, not a broken build.
func TestUnknownPropsAreANote(t *testing.T) {
	r := scan(t)
	var chart Island
	for _, is := range r.Islands {
		if is.Src == "/chart.js" {
			chart = is
		}
	}
	if chart.Src == "" || chart.Type != "" {
		t.Fatalf("the map literal should have no type: %+v", chart)
	}
	if len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "/chart.js") {
		t.Fatalf("notes: %v", r.Notes)
	}
	ts := r.TypeScript()
	if !strings.Contains(ts, `"/chart.js": unknown;`) {
		t.Errorf("the island is still declared:\n%s", ts)
	}
	if !strings.Contains(ts, "// note: ") {
		t.Errorf("the note is not in the file:\n%s", ts)
	}
}

// An island with no props at all is not a mistake, so it gets no note.
func TestNilPropsIsNotANote(t *testing.T) {
	ts := scan(t).TypeScript()
	if !strings.Contains(ts, `"/clock.js": unknown;`) {
		t.Errorf("the island with nil props is missing:\n%s", ts)
	}
	if strings.Contains(ts, "/clock.js are not a struct") {
		t.Error("nil props got a note it did not deserve")
	}
}

// The file describes the island object too, so the module can be checked
// without the app declaring the framework's own contract.
func TestRuntimeIsDeclared(t *testing.T) {
	ts := scan(t).TypeScript()
	for _, want := range []string{
		"export interface TrilhaIsland {",
		"csrf(): string;",
		"readonly signal: AbortSignal;",
		"post<T = unknown>(url: string, data?: unknown): Promise<T>;",
		"swap(url: string, id?: string): Promise<boolean>;",
		"export interface IslandInvalid extends IslandError {",
		"readonly fields: Record<string, string>;",
		"export type IslandMount<S extends keyof TrilhaIslandProps> = (",
		Header,
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("missing %q", want)
		}
	}
}

// An app with no app/ directory has nothing to declare, and that is not an error.
func TestMissingAppDir(t *testing.T) {
	r, err := Scan("../../testdata/islands/nope")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Islands) != 0 {
		t.Fatalf("islands: %v", r.Islands)
	}
	if !strings.Contains(r.TypeScript(), "[src: string]: unknown;") {
		t.Error("an empty map should still let a module be typed")
	}
}
