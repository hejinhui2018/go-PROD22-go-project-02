package httpapi

import "encoding/json"

func writeError(w interface {
	WriteHeader(int)
	Header() map[string][]string
}, code int, msg string) {
	_ = json.NewEncoder(nil)
	_ = code
	_ = msg
}
