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
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// NewJitteredTimer returns a channel which will be written to on every tick
// of a jittered timer.
func NewJitteredTimer(duration time.Duration, factor float64) <-chan struct{} {
	ch := make(chan struct{})
	go wait.JitterUntil(func() {
		ch <- struct{}{}
	}, duration, factor, false, wait.NeverStop)
	return ch
}
