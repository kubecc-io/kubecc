package types

import (
	"github.com/cobalt77/kubecc/internal/zapkc"
)

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
		return "kcctl"
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
