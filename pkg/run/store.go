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
