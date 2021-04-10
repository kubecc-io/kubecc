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

package types

import (
	"errors"
	"strings"

	"github.com/kubecc-io/kubecc/pkg/util"
	md5simd "github.com/minio/md5-simd"
)

// EquivalentTo Compares all fields except executable, because
// the binaries can live in different locations on different systems.
func (tc *Toolchain) EquivalentTo(other *Toolchain) bool {
	return tc.GetKind() == other.GetKind() &&
		tc.GetLang() == other.GetLang() &&
		tc.GetTargetArch() == other.GetTargetArch() &&
		tc.GetPicDefault() == other.GetPicDefault() &&
		tc.GetVersion() == other.GetVersion()
}

// Canonical returns a string identifying the key with the associated bucket.
func (k *Key) Canonical() string {
	return k.Bucket + "." + k.Name
}

func (k *Key) ShortID() string {
	return FormatShortID(k.Bucket, 6, ElideCenter) + "." + k.Name
}

func (tc *Toolchain) Hash(hasher md5simd.Hasher) {
	util.Must(hasher.Write([]byte(tc.TargetArch)))
	util.Must(hasher.Write([]byte(ToolchainKind_name[int32(tc.Kind)])))
	util.Must(hasher.Write([]byte(ToolchainLang_name[int32(tc.Lang)])))
	util.Must(hasher.Write([]byte(tc.Version)))
	if tc.PicDefault {
		util.Must(hasher.Write([]byte{1}))
	} else {
		util.Must(hasher.Write([]byte{0}))
	}
}

func (req *CompileRequest) Hash(hasher md5simd.Hasher) {
	req.Toolchain.Hash(hasher)
	util.Must(hasher.Write(req.PreprocessedSource))
	for _, arg := range req.Args {
		util.Must(hasher.Write([]byte(arg)))
	}
}

var (
	ErrInvalidFormat = errors.New("Invalid key format, should be of the form bucket.name")
)

func ParseKey(canonical string) (*Key, error) {
	split := strings.SplitN(canonical, ".", 2)
	if len(split) != 2 {
		return nil, ErrInvalidFormat
	}
	return &Key{
		Bucket: split[0],
		Name:   split[1],
	}, nil
}

func (tc *Toolchain) CommonNames() []string {
	switch tc.Kind {
	case Clang:
		switch tc.Lang {
		case C:
			return []string{"clang"}
		case CXX:
			return []string{"clang++"}
		case Multi:
			return []string{"clang", "clang++"}
		}
	case Gnu:
		switch tc.Lang {
		case C:
			return []string{"gcc", "cc"}
		case CXX:
			return []string{"g++"}
		case Multi:
			return []string{"gcc", "g++"}
		}
	}
	return []string{}
}
