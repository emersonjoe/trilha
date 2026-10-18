// Package ilhas has twenty islands on one page, and counts the observers of
// the whole document the page ends up with (spec 177).
package ilhas

import (
	"fmt"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// spy counts every observe() on the whole document. It runs before any
// deferred script, so it sees the island runtime's.
const spy = `(() => {
  window.__docObservers = 0;
  const observe = MutationObserver.prototype.observe;
  MutationObserver.prototype.observe = function (target, opts) {
    if (target === document && opts && opts.subtree) window.__docObservers++;
    return observe.call(this, target, opts);
  };
})();`

func Page(c *trilha.Ctx) (h.Node, error) {
	islands := make([]h.Node, 0, 20)
	for i := 0; i < 20; i++ {
		islands = append(islands, c.Island("/ilha-contador.js", map[string]int{"start": i},
			h.P(h.ID(fmt.Sprintf("contador-%d", i)), h.Text("fallback"))))
	}
	return h.Div(h.Script(trilha.NonceAttr(c), h.Raw(spy)), h.H1(h.Text("Islands")), h.Div(islands...)), nil
}
