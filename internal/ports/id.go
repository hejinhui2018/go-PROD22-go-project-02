package ports

import (
	"fmt"
	"sync/atomic"
	"time"
)

type IDGenerator interface{ New(prefix string) string }
type SequenceID struct{ n uint64 }

func (s *SequenceID) New(p string) string {
	n := atomic.AddUint64(&s.n, 1)
	return fmt.Sprintf("%s-%d-%d", p, time.Now().UnixNano(), n)
}
