package archive

import (
	"fmt"
	"time"
)

// Window limits an export to campaigns from the last Years years. A zero or
// negative Years keeps every campaign.
type Window struct {
	Years int
	Now   time.Time
}

// Cutoff returns the oldest date inside the window, or the zero time when the
// window is unbounded.
func (w Window) Cutoff() time.Time {
	if w.Years <= 0 {
		return time.Time{}
	}

	return w.Now.AddDate(-w.Years, 0, 0)
}

// SkipReason explains why a campaign date is outside the window, or returns an
// empty string when it belongs in the export.
func (w Window) SkipReason(date time.Time, valid bool) string {
	if !valid {
		return "no usable campaign date"
	}

	cutoff := w.Cutoff()
	if cutoff.IsZero() || !date.Before(cutoff) {
		return ""
	}

	return fmt.Sprintf("dated %s, older than %d years", date.UTC().Format("2006-01-02"), w.Years)
}
