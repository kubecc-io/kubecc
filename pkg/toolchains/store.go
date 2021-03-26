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

package toolchains

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

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

// NewStore creates a new toolchain store.
func NewStore() *Store {
	return &Store{
		toolchains: make(map[string]*toolchainData),
		tcMutex:    &sync.RWMutex{},
	}
}

// Contains checks whether the given executable is contained in the
// store. It checks the full path given, and does not modify it.
func (s *Store) Contains(executable string) bool {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	executable = evalPath(executable)
	_, ok := s.toolchains[executable]
	return ok
}

// Items returns an iteratable channel which will contain all toolchains
// in the store. This function is thread safe and the toolchains returned
// will be copies.
func (s *Store) Items() chan *types.Toolchain {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	ch := make(chan *types.Toolchain, len(s.toolchains))
	for _, data := range s.toolchains {
		ch <- proto.Clone(data.toolchain).(*types.Toolchain)
	}
	close(ch)
	return ch
}

// Items returns a slice containing all toolchains in the store.
// This function is thread safe and the toolchains returned will be copies.
func (s *Store) ItemsList() []*types.Toolchain {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	l := []*types.Toolchain{}
	for _, data := range s.toolchains {
		l = append(l, proto.Clone(data.toolchain).(*types.Toolchain))
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

// Add adds a toolchain to the store. The executable given will be queried
// for details using the given Querier.
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

// Find returns a toolchain matching the given executable if it is contained
// in the store. If the path is a symlink, it will be dereferenced.
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

// Remove removes a toolchain from the store.
func (s *Store) Remove(tc *types.Toolchain) {
	s.tcMutex.Lock()
	defer s.tcMutex.Unlock()

	delete(s.toolchains, tc.Executable)
}

// UpdateIfNeeded will reload a toolchain's details be querying the
// executable using the original Querier. If the toolchain or store was
// updated as a result of this operation, the second return value will
// be true, otherwise it will be false. If the toolchain is no longer valid,
// the first return value will contain an error describing why.
func (s *Store) UpdateIfNeeded(tc *types.Toolchain) (error, bool) {
	s.tcMutex.Lock()
	defer s.tcMutex.Unlock()

	data := s.toolchains[tc.Executable]
	timestamp, err := data.querier.ModTime(tc.Executable)
	if _, ok := err.(*fs.PathError); ok {
		// Executable no longer exists; remove toolchain
		delete(s.toolchains, tc.Executable)
		return err, true
	}

	if timestamp != data.modTime {
		err := fillInfo(tc, data.querier)
		if err != nil {
			// Toolchain became invalid
			return err, true
		}
		data.modTime = timestamp
		// Updated successfully
		return nil, true
	}

	return nil, false
}

// Merge adds toolchains from another store into this one, if they do not exist.
func (s *Store) Merge(other *Store) {
	other.tcMutex.RLock()
	defer other.tcMutex.RUnlock()

	for tc := range other.Items() {
		if s.Contains(tc.Executable) {
			continue
		}
		s.Add(tc.Executable, other.toolchains[tc.Executable].querier)
	}
}

// Intersection returns the toolchains that both this store and the other
// store have in common.
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

// Len returns the number of toolchains in the store.
func (s *Store) Len() int {
	s.tcMutex.RLock()
	defer s.tcMutex.RUnlock()

	return len(s.toolchains)
}

// TryMatch will attempt to find a toolchain in this store that matches
// the provided toolchain, assuming the other toolchain may have come from
// a different store where the executable name might not match exactly.
func (s *Store) TryMatch(other *types.Toolchain) (*types.Toolchain, error) {
	for tc := range s.Items() {
		if tc.EquivalentTo(other) {
			return tc, nil
		}
	}
	return nil, fmt.Errorf("No local toolchain matches remote")
}
