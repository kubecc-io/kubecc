package util

import (
	"sync"
)

func Must(arg interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return arg
}

// NullableError is a thread-safe wrapper around an error that ensures an
// error has been explicitly set, regardless of value, before being checked.
type NullableError struct {
	mu    sync.Mutex
	isSet bool
	err   error
}

// SetErr sets the error value and allows Err to be called.
func (ae *NullableError) SetErr(err error) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	ae.isSet = true
	ae.err = err
}

// Err returns the error value stored in the NullableError. If the error has
// not been set using the Set method, this function will panic.
func (ae *NullableError) Err() error {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	if !ae.isSet {
		panic("Error has not been set")
	}
	return ae.err
}
