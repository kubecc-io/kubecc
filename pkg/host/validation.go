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
	"go.uber.org/zap"
	validators "k8s.io/system-validators/validators"
)

func RunPreflightChecks(lg *zap.SugaredLogger) {
	lg.Debug("Running preflight checks")
	warns, errs := (&validators.KernelValidator{}).Validate(validators.SysSpec{
		OS: "Linux",
		KernelSpec: validators.KernelSpec{
			Versions: []string{`^3\.[1-9][0-9].*$`, `^([4-9]|[1-9][0-9]+)\.([0-9]+)\.([0-9]+).*$`}, // Requires 3.10+, or newer,
			Optional: []validators.KernelConfig{
				{Name: "CFS_BANDWIDTH", Description: "Required for CPU quota."},
			},
		},
	})

	if len(warns) == 0 && len(errs) == 0 {
		lg.Debug("Checks passed")
	}

	for _, w := range warns {
		lg.Warn(w)
	}
	for _, e := range errs {
		lg.Error(e)
	}
}
