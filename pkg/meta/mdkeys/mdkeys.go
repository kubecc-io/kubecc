package mdkeys

type componentKeyType struct{}
type uuidKeyType struct{}
type logKeyType struct{}
type tracingKeyType struct{}
type systemInfoKeyType struct{}

func (componentKeyType) String() string {
	return "kubecc-component"
}

func (uuidKeyType) String() string {
	return "kubecc-uuid"
}

func (logKeyType) String() string {
	return "kubecc-log"
}

func (tracingKeyType) String() string {
	return "kubecc-tracing"
}

func (systemInfoKeyType) String() string {
	return "kubecc-systeminfo"
}

var (
	ComponentKey  componentKeyType
	UUIDKey       uuidKeyType
	LogKey        logKeyType
	TracingKey    tracingKeyType
	SystemInfoKey systemInfoKeyType
)
