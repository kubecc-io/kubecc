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

package monitor_test

import (
	"testing"
	"time"

	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var testLog *zap.SugaredLogger

func TestMonitor(t *testing.T) {
	testLog = logkc.New(types.TestComponent, logkc.WithWriter(GinkgoWriter))
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(3 * time.Second)
	SetDefaultEventuallyPollingInterval(50 * time.Millisecond)
	RunSpecs(t, "Monitor Suite")
}
