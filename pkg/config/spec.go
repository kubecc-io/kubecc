package config

import (
	"fmt"
	"reflect"

	"go.uber.org/zap/zapcore"
)

type logLevelString string

func (str logLevelString) Level() (l zapcore.Level) {
	if err := l.Set(string(str)); err != nil {
		panic(fmt.Sprintf("Could not parse log level string %q", str))
	}
	return
}

type GlobalSpec struct {
	LogLevel logLevelString `json:"logLevel"`
	LogFile  string         `json:"logFile"`
}

// LoadIfUnset will load fields from the global GlobalSpec into
// a component's optional GlobalSpec if a field has not been
// specifically overridden by the component.
func (override GlobalSpec) LoadIfUnset(global GlobalSpec) {
	overrideValue := reflect.ValueOf(override)
	globalValue := reflect.ValueOf(global)
	for i := 0; i < overrideValue.NumField(); i++ {
		if field := overrideValue.Field(i); !field.IsValid() {
			globalValue.Field(i).Set(field)
		}
	}
}

type KubeccSpec struct {
	Global    GlobalSpec    `json:"global"`
	Agent     AgentSpec     `json:"agent"`
	Consumer  ConsumerSpec  `json:"consumer"`
	Consumerd ConsumerdSpec `json:"consumerd"`
	Scheduler SchedulerSpec `json:"scheduler"`
	Monitor   MonitorSpec   `json:"monitor"`
	Cache     CacheSpec     `json:"cache"`
	Kcctl     KcctlSpec     `json:"kcctl"`
}

type AgentSpec struct {
	GlobalSpec
	UsageLimits      UsageLimitsSpec `json:"usageLimits"`
	SchedulerAddress string          `json:"schedulerAddress"`
	MonitorAddress   string          `json:"monitorAddress"`
	ListenAddress    string          `json:"listenAddress"`
}

type ConsumerSpec struct {
	GlobalSpec
	ConsumerdAddress string `json:"consumerdAddress"`
}

type ConsumerdSpec struct {
	GlobalSpec
	UsageLimits      UsageLimitsSpec `json:"usageLimits"`
	SchedulerAddress string          `json:"schedulerAddress"`
	MonitorAddress   string          `json:"monitorAddress"`
	ListenAddress    string          `json:"listenAddress"`
	DisableTLS       bool            `json:"disableTLS"`
}

type SchedulerSpec struct {
	GlobalSpec
	MonitorAddress string `json:"monitorAddress"`
	CacheAddress   string `json:"cacheAddress"`
	ListenAddress  string `json:"listenAddress"`
}

type MonitorSpec struct {
	GlobalSpec
	ListenAddress string `json:"listenAddress"`
}

type CacheSpec struct {
	GlobalSpec
	LocalStorage   *LocalStorageSpec  `json:"localStorage"`
	RemoteStorage  *RemoteStorageSpec `json:"remoteStorage"`
	ListenAddress  string             `json:"listenAddress"`
	MonitorAddress string             `json:"monitorAddress"`
}

type LocalStorageSpec struct {
	Limits StorageLimitsSpec `json:"limits"`
	Path   string            `json:"path"`
}

type RemoteStorageSpec struct {
	Endpoint       string `json:"endpoint"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	TLS            bool   `json:"tls"`
	CertPath       string `json:"certPath"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	ExpirationDays int    `json:"expirationDays"`
}

type StorageLimitsSpec struct {
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

type KcctlSpec struct {
	GlobalSpec
	MonitorAddress string `json:"monitorAddress"`
	DisableTLS     bool   `json:"disableTLS"`
}

type UsageLimitsSpec struct {
	QueuePressureMultiplier float64 `json:"queuePressureMultiplier"`
	QueueRejectMultiplier   float64 `json:"queueRejectMultiplier"`
	ConcurrentProcessLimit  int     `json:"concurrentProcessLimit"`
}
