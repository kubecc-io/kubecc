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

package run

import "github.com/cobalt77/kubecc/pkg/types"

type ToolchainRunnerStore struct {
	items map[types.ToolchainKind]ToolchainController
}
type StoreAddFunc func(*ToolchainRunnerStore)

func NewToolchainRunnerStore() *ToolchainRunnerStore {
	return &ToolchainRunnerStore{
		items: make(map[types.ToolchainKind]ToolchainController),
	}
}

type NoRunnerForKind struct{}

func (e NoRunnerForKind) Error() string {
	panic("No runner available")
}

func (s *ToolchainRunnerStore) Add(
	kind types.ToolchainKind,
	runner ToolchainController,
) {
	if _, ok := s.items[kind]; ok {
		panic("Tried to add an already existing runner")
	}
	s.items[kind] = runner
}

func (s *ToolchainRunnerStore) Get(
	kind types.ToolchainKind,
) (ToolchainController, error) {
	if r, ok := s.items[kind]; ok {
		return r, nil
	}
	return nil, &NoRunnerForKind{}
}
