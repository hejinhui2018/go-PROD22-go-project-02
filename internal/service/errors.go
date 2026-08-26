package service

import "errors"

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("state conflict")
var ErrNoTask = errors.New("no task available")
