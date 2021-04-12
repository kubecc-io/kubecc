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

import "errors"

type CompilerError struct {
	error
	text string
}

func NewCompilerError(text string) *CompilerError {
	return &CompilerError{
		text: text,
	}
}

func (e *CompilerError) Error() string {
	return e.text
}

func IsCompilerError(err error) bool {
	var e *CompilerError
	return errors.As(err, &e)
}

var ErrNoAgentsRetry = errors.New("No agents available to handle the request; retrying")
var ErrNoAgentsRunLocal = errors.New("No agents available to handle the request; running locally")
