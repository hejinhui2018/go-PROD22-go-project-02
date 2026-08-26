package ports

import "time"

type FakeClock struct{ T time.Time }

func NewFakeClock() *FakeClock               { return &FakeClock{T: time.Unix(0, 0).UTC()} }
func (c *FakeClock) Now() time.Time          { return c.T }
func (c *FakeClock) Advance(d time.Duration) { c.T = c.T.Add(d) }
