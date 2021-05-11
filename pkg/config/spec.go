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

type LogLevelString string

func (str LogLevelString) Level() (l zapcore.Level) {
	if err := l.Set(string(str)); err != nil {
		panic(fmt.Sprintf("Could not parse log level string %q", str))
	}
	return
}

type GlobalSpec struct {
	LogLevel LogLevelString `json:"logLevel,omitempty"`
	LogFile  string         `json:"logFile,omitempty"`
}

// Merge will load fields from the global GlobalSpec into
// a component's optional GlobalSpec if a field has not been
// specifically overridden by the component.
func (override *GlobalSpec) Merge(global GlobalSpec) {
	if err := mergo.Merge(override, global); err != nil {
		panic(err)
	}
}

type KubeccSpec struct {
	Global    GlobalSpec    `json:"global,omitempty"`
	Agent     AgentSpec     `json:"agent,omitempty"`
	Consumer  ConsumerSpec  `json:"consumer,omitempty"`
	Consumerd ConsumerdSpec `json:"consumerd,omitempty"`
	Scheduler SchedulerSpec `json:"scheduler,omitempty"`
	Monitor   MonitorSpec   `json:"monitor,omitempty"`
	Cache     CacheSpec     `json:"cache,omitempty"`
	Kcctl     KcctlSpec     `json:"kcctl,omitempty"`
}

type AgentSpec struct {
	GlobalSpec
	UsageLimits      *UsageLimitsSpec `json:"usageLimits,omitempty"`
	SchedulerAddress string           `json:"schedulerAddress,omitempty"`
	MonitorAddress   string           `json:"monitorAddress,omitempty"`
}

type ConsumerSpec struct {
	GlobalSpec
	ConsumerdAddress string `json:"consumerdAddress,omitempty"`
}

type ConsumerdSpec struct {
	GlobalSpec
	UsageLimits      *UsageLimitsSpec `json:"usageLimits,omitempty"`
	SchedulerAddress string           `json:"schedulerAddress,omitempty"`
	MonitorAddress   string           `json:"monitorAddress,omitempty"`
	ListenAddress    string           `json:"listenAddress,omitempty"`
	DisableTLS       bool             `json:"disableTLS,omitempty"`
}

type SchedulerSpec struct {
	GlobalSpec
	MonitorAddress string `json:"monitorAddress,omitempty"`
	CacheAddress   string `json:"cacheAddress,omitempty"`
	ListenAddress  string `json:"listenAddress,omitempty"`
}

type MonitorSpec struct {
	GlobalSpec
	ListenAddress          string `json:"listenAddress,omitempty"`
	ServePrometheusMetrics bool   `json:"servePrometheusMetrics,omitempty"`
}

type CacheSpec struct {
	GlobalSpec
	LocalStorage   *LocalStorageSpec  `json:"localStorage,omitempty"`
	RemoteStorage  *RemoteStorageSpec `json:"remoteStorage,omitempty"`
	ListenAddress  string             `json:"listenAddress,omitempty"`
	MonitorAddress string             `json:"monitorAddress,omitempty"`
}

type LocalStorageSpec struct {
	Limits StorageLimitsSpec `json:"limits,omitempty"`
	Path   string            `json:"path,omitempty"`
}

type RemoteStorageSpec struct {
	Endpoint       string `json:"endpoint,omitempty"`
	AccessKey      string `json:"accessKey,omitempty"`
	SecretKey      string `json:"secretKey,omitempty"`
	TLS            bool   `json:"tls,omitempty"`
	CertPath       string `json:"certPath,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	Region         string `json:"region,omitempty"`
	ExpirationDays int    `json:"expirationDays,omitempty"`
}

type StorageLimitsSpec struct {
	Memory string `json:"memory,omitempty"`
	Disk   string `json:"disk,omitempty"`
}

type KcctlSpec struct {
	GlobalSpec
	MonitorAddress   string `json:"monitorAddress,omitempty"`
	SchedulerAddress string `json:"schedulerAddress,omitempty"`
	DisableTLS       bool   `json:"disableTLS,omitempty"`
}

type UsageLimitsSpec struct {
	ConcurrentProcessLimit int `json:"concurrentProcessLimit,omitempty"`
}

func (ul *UsageLimitsSpec) GetConcurrentProcessLimit() int {
	if ul == nil {
		return -1
	}
	return ul.ConcurrentProcessLimit
}
