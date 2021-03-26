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

package config

import (
	"fmt"

	"github.com/imdario/mergo"
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
func (override *GlobalSpec) LoadIfUnset(global GlobalSpec) {
	if err := mergo.Merge(override, global); err != nil {
		panic(err)
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
	ListenAddress          string `json:"listenAddress"`
	ServePrometheusMetrics bool   `json:"servePrometheusMetrics"`
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
	MonitorAddress   string `json:"monitorAddress"`
	SchedulerAddress string `json:"schedulerAddress"`
	DisableTLS       bool   `json:"disableTLS"`
}

type UsageLimitsSpec struct {
	QueuePressureMultiplier float64 `json:"queuePressureMultiplier"`
	QueueRejectMultiplier   float64 `json:"queueRejectMultiplier"`
	ConcurrentProcessLimit  int     `json:"concurrentProcessLimit"`
}
