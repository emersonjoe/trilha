package trilha

import (
	"strconv"
	"time"
)

// pollHeader is how the route talks back to a polling fragment: how long to
// wait for the next one, or that there is nothing left to wait for.
const pollHeader = "Trilha-Poll"

// PollEvery changes the interval of the fragment that is polling this route.
// A job that starts slow and ends fast can ask for one rhythm and then
// another; the client obeys the last answer it got.
//
//	if doc.Stage == "uploading" {
//		c.PollEvery(2 * time.Second)
//	}
func (c *Ctx) PollEvery(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	c.w.Header().Set(pollHeader, strconv.Itoa(int(d.Round(time.Second)/time.Second))+"s")
}

// PollStop ends the polling of the fragment that asked: the document is
// processed, the batch is over, there is nothing else to watch. The answer is
// still the fragment, so the last state is the one that stays on the screen.
//
//	if doc.Done() {
//		c.PollStop()
//	}
//	return c.Render(200, statusFragment(doc))
func (c *Ctx) PollStop() { c.w.Header().Set(pollHeader, "stop") }
