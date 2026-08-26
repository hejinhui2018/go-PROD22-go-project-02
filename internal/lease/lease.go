package lease

import "time"

type Lease struct {
	Owner    string
	Until    time.Time
	Attempts int
}

func (l Lease) Expired(now time.Time) bool { return !l.Until.After(now) }
