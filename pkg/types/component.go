package types

import "github.com/cobalt77/kubecc/internal/zapkc"

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
	case TestComponent:
		return "Test"
	}
	return "Unknown"
}

func (c Component) ShortName() string {
	switch c {
	case Agent:
		return "agnt"
	case Scheduler:
		return "schd"
	case Controller:
		return "ctrl"
	case Consumer:
		return "csmr"
	case Consumerd:
		return "csrd"
	case Make:
		return "make"
	case CLI:
		return " cli"
	case Monitor:
		return "mntr"
	case Dashboard:
		return "dash"
	case TestComponent:
		return "test"
	default:
	}
	return "????"
}

func (c Component) Color() zapkc.Color {
	switch c {
	case Agent:
		return zapkc.Magenta
	case Scheduler:
		return zapkc.Yellow
	case Controller:
		return zapkc.Blue
	case Consumerd:
		return zapkc.Green
	case Make:
		return zapkc.NoColor
	case CLI:
		return zapkc.Cyan
	case Monitor:
		return zapkc.Red
	case TestComponent, Consumer, Dashboard:
		return zapkc.White
	}
	return zapkc.NoColor
}
