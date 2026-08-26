package backoff

import (
	"testing"
	"time"
)

func TestDelay(t *testing.T) {
	if Delay(2) != 2*time.Minute {
		t.Fatal(Delay(2))
	}
}
