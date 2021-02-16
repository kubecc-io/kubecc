package toolchains

import (
	"io/fs"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/cobalt77/kubecc/pkg/types"
)

type toolchainData struct {
	toolchain *types.Toolchain
	modTime   time.Time
	querier   Querier
}

// Store stores toolchains and provides ways to access them
type Store struct {
	toolchains map[string]*toolchainData
}

func NewStore() *Store {
	return &Store{
		toolchains: make(map[string]*toolchainData),
	}
}

func (s Store) Contains(executable string) bool {
	executable = evalPath(executable)

	_, ok := s.toolchains[executable]
	return ok
}

func (s Store) Items() chan *types.Toolchain {
	ch := make(chan *types.Toolchain, len(s.toolchains))
	for _, data := range s.toolchains {
		ch <- data.toolchain
	}
	close(ch)
	return ch
}

func (s Store) ItemsList() []*types.Toolchain {
	l := []*types.Toolchain{}
	for _, data := range s.toolchains {
		l = append(l, data.toolchain)
	}
	return l
}

func fillInfo(tc *types.Toolchain, q Querier) error {
	var err error
	tc.TargetArch, err = q.TargetArch(tc.Executable)
	if err != nil {
		return errors.WithMessage(err, "Could not determine target arch")
	}
	tc.Version, err = q.Version(tc.Executable)
	if err != nil {
		return errors.WithMessage(err, "Could not determine target version")
	}
	tc.PicDefault, err = q.IsPicDefault(tc.Executable)
	if err != nil {
		return errors.WithMessage(err, "Could not determine compiler PIC defaults")
	}
	tc.Kind, err = q.Kind(tc.Executable)
	if err != nil {
		return errors.WithMessage(err, "Could not determine compiler kind (gcc/clang)")
	}
	tc.Lang, err = q.Lang(tc.Executable)
	if err != nil {
		return errors.WithMessage(err, "Could not determine compiler language (c/cxx/multi)")
	}
	return nil
}

func (s *Store) Add(executable string, q Querier) (*types.Toolchain, error) {
	executable = evalPath(executable)

	if s.Contains(executable) {
		return nil, errors.New("Tried to add an already-existing toolchain")
	}
	tc := &types.Toolchain{
		Executable: executable,
	}
	err := fillInfo(tc, q)
	if err != nil {
		return nil, err
	}
	modTime, err := q.ModTime(executable)
	if err != nil {
		return nil, errors.WithMessage(err, "Could not determine compiler modification time")
	}
	s.toolchains[executable] = &toolchainData{
		toolchain: tc,
		modTime:   modTime,
		querier:   q,
	}
	return tc, nil
}

func evalPath(executable string) string {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return executable
	}
	return resolved
}

func (s Store) Find(executable string) (*types.Toolchain, error) {
	if data, ok := s.toolchains[executable]; ok {
		return data.toolchain, nil
	} else if data, ok := s.toolchains[evalPath(executable)]; ok {
		return data.toolchain, nil
	}

	return nil, errors.New("Toolchain not found")
}

func (s *Store) Remove(tc *types.Toolchain) {
	delete(s.toolchains, tc.Executable)
}

func (s *Store) UpdateIfNeeded(tc *types.Toolchain) error {
	data := s.toolchains[tc.Executable]
	timestamp, err := data.querier.ModTime(tc.Executable)
	if _, is := err.(*fs.PathError); is {
		// Executable no longer exists; remove toolchain
		s.Remove(tc)
		return err
	}

	if timestamp != data.modTime {
		err := fillInfo(tc, data.querier)
		if err != nil {
			// Toolchain became invalid
			return err
		}
	}
	return nil
}
