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
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/pkg/types"
)

var _ = Describe("Loghelpers", func() {
	Context("ID formatting", func() {
		id := uuid.NewString()
		Specify("FormatShortID should format correctly", func() {
			Expect(types.FormatShortID(id, 6, types.ElideCenter)).To(Equal(
				id[:3] + ".." + id[len(id)-3:],
			))
			Expect(types.FormatShortID(id, 4, types.ElideCenter)).To(Equal(
				id[:2] + ".." + id[len(id)-2:],
			))
			Expect(types.FormatShortID(id, 2, types.ElideCenter)).To(Equal(
				id[:1] + ".." + id[len(id)-1:],
			))
			Expect(types.FormatShortID(id, 6, types.ElideLeft)).To(Equal(
				".." + id[len(id)-5:],
			))
			Expect(types.FormatShortID(id, 6, types.ElideRight)).To(Equal(
				id[:6] + "..",
			))
		})
	})
})
