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

package clients

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type noopProvider struct{}

func NewNoopMetricsProvider() MetricsProvider {
	return &noopProvider{}
}

func (noopProvider) Post(proto.Message) {}

func (noopProvider) PostContext(proto.Message, context.Context) {}

type noopListener struct{}

func NewNoopMetricsListener() MetricsListener {
	return &noopListener{}
}

func (noopListener) OnValueChanged(string, interface{}) ChangeListener {
	return noopChangeListener{}
}

func (noopListener) OnProviderAdded(func(context.Context, string)) {}

func (noopListener) Stop() {}

type noopChangeListener struct{}

func (noopChangeListener) TryConnect() (grpc.ClientStream, error) {
	return nil, nil
}

func (noopChangeListener) HandleStream(grpc.ClientStream) error {
	return nil
}

func (noopChangeListener) Target() string {
	return "<unknown>"
}

func (noopChangeListener) OrExpired(func() RetryOptions) {}
