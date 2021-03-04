package config

type KubeccSpec struct {
	Agent     AgentSpec     `json:"agent"`
	Consumer  ConsumerSpec  `json:"consumer"`
	Consumerd ConsumerdSpec `json:"consumerd"`
	Scheduler SchedulerSpec `json:"scheduler"`
	Monitor   MonitorSpec   `json:"monitor"`
	Cache     CacheSpec     `json:"cache"`
	Kcctl     KcctlSpec     `json:"kcctl"`
}

type AgentSpec struct {
	UsageLimits      UsageLimitsSpec `json:"usageLimits,omitempty"`
	SchedulerAddress string          `json:"schedulerAddress"`
	MonitorAddress   string          `json:"monitorAddress"`
	ListenAddress    string          `json:"listenAddress"`
	LogLevel         string          `json:"logLevel,omitempty"`
}

type ConsumerSpec struct {
	ConsumerdAddress string `json:"consumerdAddress"`
	LogLevel         string `json:"logLevel,omitempty"`
	LogFile          string `json:"logFile,omitempty"`
}

type ConsumerdSpec struct {
	UsageLimits      UsageLimitsSpec `json:"usageLimits,omitempty"`
	SchedulerAddress string          `json:"schedulerAddress"`
	MonitorAddress   string          `json:"monitorAddress"`
	ListenAddress    string          `json:"listenAddress"`
	LogLevel         string          `json:"logLevel"`
	DisableTLS       bool            `json:"disableTLS,omitempty"`
}

type SchedulerSpec struct {
	MonitorAddress string `json:"monitorAddress"`
	ListenAddress  string `json:"listenAddress"`
	LogLevel       string `json:"logLevel,omitempty"`
}

type MonitorSpec struct {
	ListenAddress MonitorListenAddressSpec `json:"listenAddress"`
	LogLevel      string                   `json:"logLevel,omitempty"`
}

type CacheSpec struct {
	LocalStorage  LocalStorageSpec  `json:"localStorage"`
	RemoteStorage RemoteStorageSpec `json:"remoteStorage,omitempty"`
	ListenAddress string            `json:"listenAddress"`
}

type LocalStorageSpec struct {
	Limits StorageLimitsSpec `json:"limits"`
	Path   string            `json:"path"`
}

type RemoteStorageSpec struct {
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	TLS       bool   `json:"tls,omitempty"`
	CertPath  string `json:"certPath,omitempty"`
	Bucket    string `json:"bucket"`
}

type StorageLimitsSpec struct {
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

type MonitorListenAddressSpec struct {
	Internal string `json:"internal"`
	External string `json:"external"`
}

type KcctlSpec struct {
	MonitorAddress string `json:"monitorAddress"`
	LogLevel       string `json:"logLevel,omitempty"`
	DisableTLS     bool   `json:"disableTLS,omitempty"`
}

type UsageLimitsSpec struct {
	QueuePressureMultiplier float64 `json:"queuePressureMultiplier,omitempty"`
	QueueRejectMultiplier   float64 `json:"queueRejectMultiplier,omitempty"`
	ConcurrentProcessLimit  int     `json:"concurrentProcessLimit,omitempty"`
}
