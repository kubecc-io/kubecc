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

package cc_test

import (
	"github.com/kubecc-io/kubecc/pkg/cc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var cases = map[string]string{
	// Basename
	"filename.i":   "c",
	"filename.ii":  "c++",
	"filename.c":   "c",
	"filename.cc":  "c++",
	"filename.cpp": "c++",
	"filename.CPP": "c++",
	"filename.cxx": "c++",
	"filename.cp":  "c++",
	"filename.c++": "c++",
	"filename.C":   "c++",
	"filename.m":   "objective-c",
	"filename.mm":  "objective-c++",
	"filename.mi":  "objective-c",
	"filename.mii": "objective-c++",
	"filename.M":   "objective-c++",
	"filename.s":   "assembler",
	"filename.S":   "assembler",
	"filename.go":  "go",
	// With directory
	"/path/to/filename.i":   "c",
	"/path/to/filename.ii":  "c++",
	"/path/to/filename.c":   "c",
	"/path/to/filename.cc":  "c++",
	"/path/to/filename.cpp": "c++",
	"/path/to/filename.CPP": "c++",
	"/path/to/filename.cxx": "c++",
	"/path/to/filename.cp":  "c++",
	"/path/to/filename.c++": "c++",
	"/path/to/filename.C":   "c++",
	"/path/to/filename.m":   "objective-c",
	"/path/to/filename.mm":  "objective-c++",
	"/path/to/filename.mi":  "objective-c",
	"/path/to/filename.mii": "objective-c++",
	"/path/to/filename.M":   "objective-c++",
	"/path/to/filename.s":   "assembler",
	"/path/to/filename.S":   "assembler",
	"/path/to/filename.go":  "go",
	// No filename
	".i":   "c",
	".ii":  "c++",
	".c":   "c",
	".cc":  "c++",
	".cpp": "c++",
	".CPP": "c++",
	".cxx": "c++",
	".cp":  "c++",
	".c++": "c++",
	".C":   "c++",
	".m":   "objective-c",
	".mm":  "objective-c++",
	".mi":  "objective-c",
	".mii": "objective-c++",
	".M":   "objective-c++",
	".s":   "assembler",
	".S":   "assembler",
	".go":  "go",
	// Not a source file
	".ccc":     "",
	".iii":     "",
	".CC":      "",
	".a":       "",
	".o":       "",
	".d":       "",
	".so":      "",
	".so.0":    "",
	".so.1":    "",
	".tar":     "",
	".gz":      "",
	".c+++":    "",
	".cs":      "",
	".rb":      "",
	".z":       "",
	".y":       "",
	".x":       "",
	".sh":      "",
	"":         "",
	"Makefile": "",
	".c.":      "",
	".cpp.":    "",
	".cppp":    "",
	".ui":      "",
	".moc":     "",
	".js":      "",
}

var _ = Describe("Filename", func() {
	It("should correctly identify source files", func() {
		for k, v := range cases {
			Expect(cc.IsSourceFile(k)).To(Equal(v != ""))
		}
	})
	It("should correctly identify source file language", func() {
		for k, v := range cases {
			lang, err := cc.SourceFileLanguage(k)
			if v == "" {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(lang).To(Equal(v))
			}
		}
	})
	It("should identify conftest files", func() {
		Expect(cc.ShouldRunLocal("conftest.c")).To(BeTrue())
		Expect(cc.ShouldRunLocal("conftest.cpp")).To(BeTrue())
		Expect(cc.ShouldRunLocal("conftest.cxx")).To(BeTrue())
		Expect(cc.ShouldRunLocal("/path/to/conftest.c")).To(BeTrue())
		Expect(cc.ShouldRunLocal("/path/to/conftest.txt")).To(BeTrue())
		Expect(cc.ShouldRunLocal("/path/to/tmp.conftest.c")).To(BeTrue())
		Expect(cc.ShouldRunLocal("tmp.conftest.c")).To(BeTrue())
	})
	It("should correctly replace file extensions", func() {
		type testCase struct {
			path, newExt, result string
		}
		cases := []testCase{
			{
				path:   "test.c",
				newExt: ".cxx",
				result: "test.cxx",
			},
			{
				path:   "/path/to/test.c",
				newExt: ".cxx",
				result: "/path/to/test.cxx",
			},
			{
				path:   "/path/to/test.c",
				newExt: ".cxx",
				result: "/path/to/test.cxx",
			},
			{
				path:   "/path/to/test.c.bak",
				newExt: ".tmp",
				result: "/path/to/test.c.tmp",
			},
			{
				path:   "a.b.c.d",
				newExt: ".e",
				result: "a.b.c.e",
			},
		}
		for _, tc := range cases {
			Expect(cc.ReplaceExtension(tc.path, tc.newExt)).To(Equal(tc.result))
		}
	})
})
