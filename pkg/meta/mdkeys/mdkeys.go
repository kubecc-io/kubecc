package mdkeys

type componentKeyType struct{}
type uuidKeyType struct{}
type logKeyType struct{}
type tracingKeyType struct{}

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

var (
	ComponentKey componentKeyType
	UuidKey      uuidKeyType
	LogKey       logKeyType
	TracingKey   tracingKeyType
)
