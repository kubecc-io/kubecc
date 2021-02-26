package toolchains

import (
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/cobalt77/kubecc/pkg/types"
)

type toolchainData struct {
	toolchain *types.Toolchain
	modTime   time.Time
	querier   Querier
}

// Store stores toolchains and provides ways to access them.
type Store struct {
	toolchains map[string]*toolchainData
	tcMutex    *sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		toolchains: make(map[string]*toolchainData),
		tcMutex:    &sync.RWMutex{},
	}
}

func (s *Store) Contains(executable string) bool {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	executable = evalPath(executable)
	_, ok := s.toolchains[executable]
	return ok
}

func (s *Store) Items() chan *types.Toolchain {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	ch := make(chan *types.Toolchain, len(s.toolchains))
	for _, data := range s.toolchains {
		ch <- data.toolchain
	}
	close(ch)
	return ch
}

func (s *Store) ItemsList() []*types.Toolchain {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

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
		return nil, errors.New("Tried to add an already existing toolchain")
	}

	s.tcMutex.Lock()
	defer s.tcMutex.Unlock()

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
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	if data, ok := s.toolchains[executable]; ok {
		return data.toolchain, nil
	} else if data, ok := s.toolchains[evalPath(executable)]; ok {
		return data.toolchain, nil
	}

	return nil, errors.New("Toolchain not found")
}

func (s *Store) Remove(tc *types.Toolchain) {
	s.tcMutex.Lock()
	defer s.tcMutex.Unlock()

	delete(s.toolchains, tc.Executable)
}

func (s *Store) UpdateIfNeeded(tc *types.Toolchain) error {
	s.tcMutex.RLock()

	data := s.toolchains[tc.Executable]
	timestamp, err := data.querier.ModTime(tc.Executable)
	var pathError *fs.PathError
	if errors.Is(err, pathError) {
		// Executable no longer exists; remove toolchain
		s.tcMutex.RUnlock()
		s.Remove(tc)
		return err
	}

	defer s.tcMutex.RUnlock()

	if timestamp != data.modTime {
		err := fillInfo(tc, data.querier)
		if err != nil {
			// Toolchain became invalid
			return err
		}
	}
	return nil
}

func (s *Store) Merge(other *Store) {
	for tc := range other.Items() {
		if s.Contains(tc.Executable) {
			continue
		}
		s.Add(tc.Executable, other.toolchains[tc.Executable].querier)
	}
}

func (s *Store) Intersection(other *Store) []*types.Toolchain {
	other.tcMutex.RLock()
	defer other.tcMutex.RUnlock()

	tcList := []*types.Toolchain{}
	for tc := range s.Items() {
		for otherTc := range other.Items() {
			if tc.EquivalentTo(otherTc) {
				tcList = append(tcList, tc)
			}
		}
	}
	return tcList
}
