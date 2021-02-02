package lll

type Component int

const (
	Agent Component = iota
	Scheduler
	Controller
	Consumer
	Consumerd
	Make
)

func (c Component) String() string {
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
	}
	return "????"
}

func (c Component) Color() Color {
	switch c {
	case Agent:
		return Magenta
	case Scheduler:
		return Yellow
	case Controller:
		return Blue
	case Consumer:
		return White
	case Consumerd:
		return Green
	case Make:
		return White
	}
	return White
}
