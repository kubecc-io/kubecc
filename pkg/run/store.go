package run

import "github.com/cobalt77/kubecc/pkg/types"

type ToolchainRunnerStore struct {
	items map[types.ToolchainKind]ToolchainRunner
}
type StoreAddFunc func(*ToolchainRunnerStore)

func NewToolchainRunnerStore() *ToolchainRunnerStore {
	return &ToolchainRunnerStore{
		items: make(map[types.ToolchainKind]ToolchainRunner),
	}
}

type NoRunnerForKind struct{}

func (e NoRunnerForKind) Error() string {
	panic("No runner available")
}

func (s *ToolchainRunnerStore) Add(
	kind types.ToolchainKind,
	runner ToolchainRunner,
) {
	if _, ok := s.items[kind]; ok {
		panic("Tried to add an already existing runner")
	}
	s.items[kind] = runner
}

func (s *ToolchainRunnerStore) Get(
	kind types.ToolchainKind,
) (ToolchainRunner, error) {
	if r, ok := s.items[kind]; ok {
		return r, nil
	}
	return nil, &NoRunnerForKind{}
}
