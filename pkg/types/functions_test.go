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

package types_test

import (
	"math/rand"

	"github.com/google/uuid"
	md5simd "github.com/minio/md5-simd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/pkg/types"
)

var _ = Describe("Functions", func() {
	Context("Toolchains", func() {
		toolchains := []*types.Toolchain{}

		Specify("setup", func() {
			for _, kind := range []types.ToolchainKind{types.Clang, types.Gnu, types.Sleep, types.TestToolchain} {
				for _, lang := range []types.ToolchainLang{types.C, types.CXX, types.Multi, types.ToolchainLang_ToolchainLang_Unknown} {
					for _, version := range []string{"7", "8", "9", "10", "10.1", "test"} {
						for _, arch := range []string{"amd64", "aarch64", "testarch"} {
							toolchains = append(toolchains, &types.Toolchain{
								Kind:       kind,
								Lang:       lang,
								TargetArch: arch,
								Version:    version,
								PicDefault: rand.Intn(2) == 1, // pic default shouldn't affect the hash
								PieDefault: rand.Intn(2) == 1, // pie default shouldn't affect the hash
								Executable: uuid.NewString(),  // executable path shouldn't affect the hash
							})
						}
					}
				}
			}
		})

		Specify("each toolchain should have a unique hash", func() {
			hashes := map[string]struct{}{}
			for _, tc := range toolchains {
				hasher := md5simd.StdlibHasher()
				tc.Hash(hasher)
				hash := string(hasher.Sum(nil))
				_, ok := hashes[hash]
				Expect(ok).To(BeFalse())
				hashes[hash] = struct{}{}
			}
		})

		Specify("each unique toolchain should only be equivalent to itself", func() {
			for i, tc := range toolchains {
				for ii, other := range toolchains {
					if i == ii {
						// should be equivalent to itself
						Expect(tc.EquivalentTo(other)).To(BeTrue())
					} else {
						// should not be equivalent to any other unique toolchain
						Expect(tc.EquivalentTo(other)).To(BeFalse())
					}
				}
			}
		})
	})

	Context("Keys", func() {
		k := &types.Key{
			Bucket: "bucket",
			Name:   "name",
		}
		Specify("key.Canonical() should properly format the key", func() {
			Expect(k.Canonical()).To(Equal("bucket.name"))
		})
		Specify("types.ParseKey() should properly parse the key", func() {
			Expect(types.ParseKey("bucket.name")).To(BeEquivalentTo(k))
		})
		Specify("error checking", func() {
			_, err := types.ParseKey("bucket")
			Expect(err).To(MatchError(types.ErrInvalidFormat))
			k, err = types.ParseKey("bucket.name.extraignoredtext")
			Expect(err).NotTo(HaveOccurred())
			Expect(k).To(BeEquivalentTo(k))
			_, err = types.ParseKey("")
			Expect(err).To(MatchError(types.ErrInvalidFormat))
			_, err = types.ParseKey("bucket,name")
			Expect(err).To(MatchError(types.ErrInvalidFormat))
		})
	})
})
