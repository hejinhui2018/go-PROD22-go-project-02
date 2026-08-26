package events

import "time"

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Aggregate string    `json:"aggregate"`
	Data      []byte    `json:"data"`
	At        time.Time `json:"at"`
}
