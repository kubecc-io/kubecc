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

package types

import (
	"github.com/kubecc-io/kubecc/internal/zapkc"
)

// Name returns the title-case component name.
func (c Component) Name() string {
	switch c {
	case Agent:
		return "Agent"
	case Scheduler:
		return "Scheduler"
	case Controller:
		return "Controller"
	case Consumer:
		return "Consumer"
	case Consumerd:
		return "Consumerd"
	case Make:
		return "Make"
	case CLI:
		return "CLI"
	case Monitor:
		return "Monitor"
	case Dashboard:
		return "Dashboard"
	case Cache:
		return "Cache"
	case TestComponent:
		return "Test"
	}
	return "Unknown"
}

// ShortName returns a lowercase, truncated name suitable for logging.
func (c Component) ShortName() string {
	switch c {
	case Agent:
		return "agent"
	case Scheduler:
		return "sched"
	case Controller:
		return "contr"
	case Consumer:
		return "consu"
	case Consumerd:
		return "consd"
	case Make:
		return "$make"
	case CLI:
		return "cli"
	case Monitor:
		return "monit"
	case Dashboard:
		return "dashb"
	case Cache:
		return "cache"
	case TestComponent:
		return "testc"
	default:
	}
	return "<unk>"
}

// Color returns a color for the component, suitable for logging.
func (c Component) Color() zapkc.Color {
	switch c {
	case Agent:
		return zapkc.Magenta
	case Scheduler:
		return zapkc.Yellow
	case Cache:
		return zapkc.Red
	case Consumerd:
		return zapkc.Green
	case Make:
		return zapkc.NoColor
	case CLI:
		return zapkc.Blue
	case Monitor:
		return zapkc.Cyan
	case TestComponent, Consumer, Dashboard, Controller:
		return zapkc.White
	}
	return zapkc.NoColor
}
