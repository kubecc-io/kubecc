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

package run

// A Resizer is an object that is able to dynamically resize its
// contained resources.
type Resizer interface {
	// Resize should set the target number of contained resources to the given
	// value. It should block until the resize operation is complete.
	Resize(int64)
}

// A ResizerManager is an object that can "take ownership" of a Resizer and
// be expected to manage its resource count.
type ResizerManager interface {
	Manage(Resizer)
}
