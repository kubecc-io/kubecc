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
	"errors"
	"strings"
)

// AnalyzeErrors analyzes error messages to find oddities and potential bugs.
func AnalyzeErrors(msg string) error {
	if strings.Contains(msg, "collect2") || strings.Contains(msg, "undefined reference") {
		return errors.New("*** BUG! Linker invoked on remote agent ***")
	}
	return nil
}
