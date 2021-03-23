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

package host

import (
	"math"
	"runtime"
)

func AutoConcurrentProcessLimit() (maxRunning int32) {
	period := CfsPeriod()
	if quota := CfsQuota(); quota < 0 {
		// No limit, default to the number of cpu threads
		maxRunning = int32(runtime.NumCPU())
	} else {
		// We need a whole number, and it needs to be at least 1.
		maxRunning = int32(math.Max(1, math.Ceil(float64(quota)/float64(period))))
	}
	return
}
