/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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
