package trilha

import (
	"sort"
	"time"
)

// Deadline is one thing with a date on it: a certificate that expires, a
// contract that renews, a document whose retention runs out.
//
// It is a plain struct and not an interface because every application already
// has its own row; what it does not have is the arithmetic below.
type Deadline struct {
	// Title is what a person reads. Kind groups — "certificate", "contract" —
	// and is what ByKind counts by.
	Title string
	Kind  string
	// Due is the day it is due. It is read as a date and not as an instant:
	// see Deadlines for what that means.
	Due time.Time
	// URL is where the thing lives. Empty draws no link, and a panel whose rows
	// link nowhere is a panel people read and then go looking.
	URL string
	// Done takes the item out of every bucket. A resolved deadline is not a
	// deadline, it is history.
	Done bool
	// Owner is who it is on, when the application tracks that. The kit shows it
	// when it is there and leaves the column out when it is not.
	Owner string
}

// DeadlineOpts configures the arithmetic.
type DeadlineOpts struct {
	// Horizons are the buckets, in days. Default: 7, 30 and 90.
	Horizons []int
	// Now is the instant "today" is read from. Zero means time.Now(), and a
	// panel that only reads the wall clock is a panel with no test.
	Now time.Time
	// In is the zone the days are counted in. Zero means Now's own zone, which
	// is what time.Now().In(c.Location()) already gives.
	In *time.Location
	// Business counts the horizons in working days instead of calendar days.
	// Nil counts calendar days.
	Business *BusinessCalendar
}

// DeadlineKind is one Kind's share of the summary.
type DeadlineKind struct {
	Kind    string
	Total   int
	Overdue int
}

// DeadlineSummary is the answer: what is late, what is close, and what is next.
type DeadlineSummary struct {
	// Overdue is everything whose day has already ended, oldest first.
	Overdue []Deadline
	// Within is cumulative: Within[30] holds everything due in the next thirty
	// days, including what is in Within[7]. That is what "within thirty days"
	// says, and a screen that wanted the ring between two horizons subtracts.
	Within map[int][]Deadline
	// Later is what is due beyond the last horizon.
	Later []Deadline
	// Next is the nearest one that has not run out yet, or nil.
	Next *Deadline
	// ByKind counts the open items per Kind. A screen that lists kinds sorts
	// them: a map has no order, and a panel whose cards move between reloads is
	// a panel people stop trusting.
	ByKind map[string]DeadlineKind
	// Total is how many items are open, and Horizons are the buckets that were
	// asked for, in the order the cards are drawn.
	Total    int
	Horizons []int
	// Now and In are what the summary was computed against, so a screen writes
	// the same "today" the arithmetic used.
	Now time.Time
	In  *time.Location
}

// Deadlines sorts a list of dates into overdue, the horizons, and what is left.
//
//	resumo := trilha.Deadlines(itens, trilha.DeadlineOpts{
//		Horizons: []int{7, 30, 90},
//		Now:      time.Now().In(c.Location()),
//	})
//	// resumo.Overdue, resumo.Within[30], resumo.Next, resumo.ByKind
//
// A deadline is a date, not an instant: something due today is not overdue
// until today is over. Comparing two time.Time values directly is the bug this
// exists to remove — it marks the morning of the due date as late, and nobody
// notices until somebody is called about a certificate that was still valid.
//
// Which day it is depends on where the reader is, so the zone is a parameter
// and not the machine's.
func Deadlines(items []Deadline, o DeadlineOpts) DeadlineSummary {
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	loc := o.In
	if loc == nil {
		loc = now.Location()
	}
	horizons := o.Horizons
	if len(horizons) == 0 {
		horizons = []int{7, 30, 90}
	} else {
		horizons = append([]int{}, horizons...)
		sort.Ints(horizons)
	}

	today := startOfDay(now, loc)
	// The edge of a horizon is the end of its last day, for the same reason
	// today's deadline is not late this morning.
	edges := make([]time.Time, len(horizons))
	for i, days := range horizons {
		edges[i] = endOfDay(o.Business.Add(today, days), loc)
	}

	s := DeadlineSummary{
		Within:   map[int][]Deadline{},
		ByKind:   map[string]DeadlineKind{},
		Horizons: horizons,
		Now:      now,
		In:       loc,
	}
	for _, h := range horizons {
		s.Within[h] = nil
	}

	open := make([]Deadline, 0, len(items))
	for _, it := range items {
		if it.Done {
			continue
		}
		open = append(open, it)
	}
	sort.SliceStable(open, func(i, j int) bool { return open[i].Due.Before(open[j].Due) })

	for _, it := range open {
		end := endOfDay(it.Due, loc)
		late := end.Before(now)
		k := s.ByKind[it.Kind]
		k.Kind, k.Total = it.Kind, k.Total+1
		if late {
			k.Overdue++
		}
		s.ByKind[it.Kind] = k

		if late {
			s.Overdue = append(s.Overdue, it)
			continue
		}
		if s.Next == nil {
			next := it
			s.Next = &next
		}
		placed := false
		for i, h := range horizons {
			if !end.After(edges[i]) {
				s.Within[h] = append(s.Within[h], it)
				placed = true
			}
		}
		if !placed {
			s.Later = append(s.Later, it)
		}
	}
	s.Total = len(open)
	return s
}

// BusinessCalendar counts days the way a working week does: weekends are not
// days, and neither are the dates the application hands over.
//
// The holidays are the application's because they are a table that changes by
// country and by year: a calendar the framework guessed would be an arithmetic
// error nobody thinks to check.
type BusinessCalendar struct {
	holidays map[string]bool
}

// BusinessDays builds the calendar.
//
//	cal := trilha.BusinessDays(feriados...)
//	prazo := cal.Add(c.Now(), 5) // five working days from today
//
// Only the calendar day of each holiday matters, so the time of day and the
// zone the dates were built in are ignored.
func BusinessDays(holidays ...time.Time) *BusinessCalendar {
	cal := &BusinessCalendar{holidays: make(map[string]bool, len(holidays))}
	for _, d := range holidays {
		cal.holidays[d.Format("2006-01-02")] = true
	}
	return cal
}

// IsBusinessDay answers whether work happens on that day.
func (c *BusinessCalendar) IsBusinessDay(t time.Time) bool {
	if c == nil {
		return true
	}
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return !c.holidays[t.Format("2006-01-02")]
}

// Add walks n working days forward from t, skipping weekends and holidays. A
// negative n walks backwards, which is how "five working days before the
// hearing" is worked out.
func (c *BusinessCalendar) Add(t time.Time, n int) time.Time {
	if c == nil {
		return t.AddDate(0, 0, n)
	}
	step := 1
	if n < 0 {
		step, n = -1, -n
	}
	for i := 0; i < n; {
		t = t.AddDate(0, 0, step)
		if c.IsBusinessDay(t) {
			i++
		}
	}
	return t
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// endOfDay is the last instant of t's calendar day, which is what a date on a
// deadline actually means.
func endOfDay(t time.Time, loc *time.Location) time.Time {
	return startOfDay(t, loc).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

// Late answers whether the day this is due on has already ended.
//
// It is a method and not a field for the reason the arithmetic above exists:
// comparing Due to now directly marks the morning of the due date as late. now
// is a parameter so a screen has a clock a test can fix.
func (d Deadline) Late(now time.Time, in *time.Location) bool {
	if d.Done || d.Due.IsZero() {
		return false
	}
	if in == nil {
		in = now.Location()
	}
	return endOfDay(d.Due, in).Before(now)
}
