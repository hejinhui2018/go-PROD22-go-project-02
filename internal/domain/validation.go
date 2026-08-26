package domain

import "fmt"

func ValidateVersion(v string) error {
	if len(v) < 3 {
		return fmt.Errorf("version must be at least 3 characters")
	}
	return nil
}
func ValidateDevices(d []string) error {
	if len(d) == 0 {
		return fmt.Errorf("at least one device required")
	}
	seen := map[string]bool{}
	for _, id := range d {
		if id == "" || seen[id] {
			return fmt.Errorf("invalid or duplicate device %q", id)
		}
		seen[id] = true
	}
	return nil
}
