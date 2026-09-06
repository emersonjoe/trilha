package trilha

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// MaxItems is how many rows or keys Bind reads into one slice or map when the
// struct does not say otherwise with maxitems. It is a ceiling against a form
// that arrives with more rows than any screen could have shown, not a limit
// anybody is meant to hit: say what the screen allows with
// `validate:"maxitems=50"` and the message reaches the person filling it in.
var MaxItems = 1000

// items is how many entries a slice of structs or a map arrived with. It is
// what minitems, maxitems and lenitems count, kept apart from a plain number
// so "min=1" on a list reads as "choose at least 1" and not as "must be 1 or
// more".
type items int

// bindSlice fills a []Struct from itens[0].nome, itens[1].nome… The rows are
// taken in numeric order and numbered again from zero, because a browser that
// removed the second of three rows sends 0 and 2, and the error has to point
// at the line the server is about to draw — not at the one the client threw
// away.
func bindSlice(fv reflect.Value, name string, f reflect.StructField, form map[string][]string, errs FieldErrors, vn *validation) {
	rows := rowsOf(form, name, limitOf(f))
	slice := reflect.MakeSlice(fv.Type(), len(rows), len(rows))
	for pos, r := range rows {
		prefix := name + "[" + strconv.Itoa(pos) + "]."
		item := make(map[string][]string, len(r.fields))
		for suffix, vals := range r.fields {
			item[prefix+suffix] = vals
		}
		bindStruct(slice.Index(pos), prefix, item, errs, vn)
	}
	fv.Set(slice)
	collect(vn, name, f, items(len(rows)))
}

// bindMap fills a map[string]T from perm[docs]=2. The key is whatever stands
// between the brackets, taken as it came: a key with a dot or a space in it is
// a key, not a path.
func bindMap(fv reflect.Value, name string, f reflect.StructField, form map[string][]string, errs FieldErrors, vn *validation) {
	if fv.Type().Key().Kind() != reflect.String {
		// A map keyed by anything else has no spelling in a form; saying so
		// here is better than filling nothing and letting the screen wonder.
		panic("trilha: Bind needs map[string]T for form field " + name + ", got " + fv.Type().String())
	}
	keys := keysOf(form, name, limitOf(f))
	m := reflect.MakeMapWithSize(fv.Type(), len(keys))
	for _, k := range keys {
		full := name + "[" + k + "]"
		ev := reflect.New(fv.Type().Elem()).Elem()
		if err := setField(ev, form[full]); err != nil {
			errs.Add(full, BindInvalid)
			vn.markBad(full)
		}
		m.SetMapIndex(reflect.ValueOf(k).Convert(fv.Type().Key()), ev)
		collect(vn, full, reflect.StructField{}, fieldValue(ev))
	}
	fv.Set(m)
	collect(vn, name, f, items(len(keys)))
}

// collect adds one more name for the validation pass to look at.
func collect(vn *validation, name string, f reflect.StructField, value any) {
	vn.fields = append(vn.fields, boundField{name: name, tag: f.Tag.Get("validate"), value: value})
}

// limitOf is how many entries may be read: what maxitems says, when it says
// less than the ceiling, plus one — so the rule still fires with a message
// instead of the list being quietly cut short.
func limitOf(f reflect.StructField) int {
	limit := MaxItems
	for _, part := range strings.Split(f.Tag.Get("validate"), ",") {
		rule, param, _ := strings.Cut(strings.TrimSpace(part), "=")
		if rule != "maxitems" && rule != "max" {
			continue
		}
		if n, err := strconv.Atoi(param); err == nil && n >= 0 && n < limit {
			limit = n + 1
		}
	}
	return limit
}

type row struct {
	index  int
	fields map[string][]string
}

// rowsOf groups the keys of one list. Nothing is allocated for an index
// nobody sent, so itens[999999999] costs one row, not a billion.
func rowsOf(form map[string][]string, name string, limit int) []row {
	byIndex := map[int]map[string][]string{}
	for k, vals := range form {
		idx, suffix, ok := cutIndex(k, name)
		if !ok || suffix == "" {
			continue
		}
		r, seen := byIndex[idx]
		if !seen {
			if len(byIndex) >= limit {
				continue
			}
			r = map[string][]string{}
			byIndex[idx] = r
		}
		r[suffix] = vals
	}
	rows := make([]row, 0, len(byIndex))
	for i, fields := range byIndex {
		rows = append(rows, row{index: i, fields: fields})
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a].index < rows[b].index })
	return rows
}

// cutIndex reads "itens[3].qtd" as 3 and "qtd", for the list called itens.
func cutIndex(key, name string) (int, string, bool) {
	if !strings.HasPrefix(key, name+"[") {
		return 0, "", false
	}
	rest := key[len(name)+1:]
	end := strings.IndexByte(rest, ']')
	if end < 1 || end > 9 { // an index nobody could have drawn is not an index
		return 0, "", false
	}
	digits := rest[:end]
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return 0, "", false
		}
	}
	idx, err := strconv.Atoi(digits)
	if err != nil {
		return 0, "", false
	}
	suffix, found := strings.CutPrefix(rest[end+1:], ".")
	if !found {
		return 0, "", false
	}
	return idx, suffix, true
}

// keysOf reads the keys of one map, in the order the messages will be read in.
func keysOf(form map[string][]string, name string, limit int) []string {
	var keys []string
	for k := range form {
		if !strings.HasPrefix(k, name+"[") || !strings.HasSuffix(k, "]") {
			continue
		}
		key := k[len(name)+1 : len(k)-1]
		if key == "" || strings.ContainsAny(key, "[]") {
			continue // a bracket inside the key is a name nobody can address
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

// bindDecoded is the same walk over a list or a map the decoder already
// filled: nothing is read here, only named and validated, so a JSON body and a
// form body answer with the same key.
func bindDecoded(fv reflect.Value, name string, f reflect.StructField, errs FieldErrors, vn *validation) {
	switch fv.Kind() {
	case reflect.Slice:
		for i := 0; i < fv.Len(); i++ {
			bindStruct(fv.Index(i), name+"["+strconv.Itoa(i)+"].", nil, errs, vn)
		}
		collect(vn, name, f, items(fv.Len()))
	case reflect.Map:
		collect(vn, name, f, items(fv.Len()))
	}
}
