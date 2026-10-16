// Package clock abstracts time so services can be tested deterministically.
package clock

import "time"

type Clock interface {
	Now() time.Time
}

// Real returns the current UTC time truncated to whole seconds, matching the
// TIMESTAMPTZ(0) precision used by every table.
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC().Truncate(time.Second) }
