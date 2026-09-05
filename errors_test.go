package trilha

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestStatusOf(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, 200},
		{ErrNotFound, 404},
		{fmt.Errorf("wrap: %w", ErrNotFound), 404},
		{Errorf(422, "bad %s", "x"), 422},
		{errors.New("boom"), 500},
	}
	for _, c := range cases {
		if got := statusOf(c.err); got != c.want {
			t.Errorf("statusOf(%v)=%d want %d", c.err, got, c.want)
		}
	}
}

func TestRedirect(t *testing.T) {
	var re *RedirectError
	if err := Redirect("/x"); !errors.As(err, &re) || re.Code != http.StatusSeeOther || re.URL != "/x" {
		t.Fatalf("unexpected %v", err)
	}
	if err := RedirectCode("/y", 301); !errors.As(err, &re) || re.Code != 301 {
		t.Fatalf("unexpected %v", err)
	}
	if got := Errorf(418, "teapot").Error(); got != "trilha: 418 teapot" {
		t.Fatal(got)
	}
}
