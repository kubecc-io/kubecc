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

package cachesrv_test

import (
	. "github.com/onsi/ginkgo"
	"go.uber.org/zap/zapcore"

	// . "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/pkg/test"
)

var _ = Describe("Cache Server", func() {
	testEnv := test.NewBufconnEnvironmentWithLogLevel(zapcore.ErrorLevel)
	Specify("setup", func() {
		test.SpawnCache(testEnv)
	})
})
