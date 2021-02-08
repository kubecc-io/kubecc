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
	case Test:
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
	case Test:
		return "test"
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
	case Consumer:
		return zapkc.White
	case Consumerd:
		return zapkc.Green
	case Make:
		return zapkc.NoColor
	case Test:
		return zapkc.White
	}
	return zapkc.NoColor
}
