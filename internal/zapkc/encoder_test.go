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

package zapkc_test

import (
	"fmt"
	"strings"

	"github.com/kubecc-io/kubecc/internal/zapkc"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gmeasure"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("Encoder", func() {
	It("should properly format entry caller", func() {
		testCases := map[string]string{
			// normal cases
			"zxxxxxx/yxxxxxxxxxxx.go:123": "yxxxxxxx+.go:123",
			"zxxxxxx/yxxxxxxxxxx.go:123":  "yxxxxxxx+.go:123",
			"zxxxxxx/yxxxxxxxxx.go:123":   "yxxxxxxx+.go:123",
			"zxxxxxx/yxxxxxxxx.go:123":    "yxxxxxxxx.go:123",
			"zxxxxxx/yxxxxxxx.go:123":     " yxxxxxxx.go:123",
			"zxxxxxx/yxxxxxx.go:123":      "z/yxxxxxx.go:123",
			"zxxxxxx/yxxxxx.go:123":       " z/yxxxxx.go:123",
			"zxxxxxx/yxxxx.go:123":        "  z/yxxxx.go:123",
			"zxxxxxx/yxxx.go:123":         "   z/yxxx.go:123",
			"zxxxxxx/yxx.go:123":          "    z/yxx.go:123",
			"zxxxxxx/yx.go:123":           "     z/yx.go:123",
			"zxxxxxx/y.go:123":            "zxxxxxx/y.go:123",
			"zxxxxx/y.go:123":             " zxxxxx/y.go:123",
			"zxxxx/y.go:123":              "  zxxxx/y.go:123",
			"zxxx/y.go:123":               "   zxxx/y.go:123",
			"zxx/y.go:123":                "    zxx/y.go:123",
			"zx/y.go:123":                 "     zx/y.go:123",
			"z/y.go:123":                  "      z/y.go:123",
			"y.go:123":                    "        y.go:123",
			"y.go:12":                     "         y.go:12",
			"y.go:1":                      "          y.go:1",
			"y.go":                        "            y.go",

			// weird cases
			"yxxxxxxxxxxxxxxxxx":    "yxxxxxxxxxxxxxx+",
			".yxxxxxxxxxxxxxxxx":    ".yxxxxxxxxxxxxx+",
			"y.yxxxxxxxxxxxxxxx":    "y.yxxxxxxxxxxxx+",
			"yxxxxxxxxxxxxxxxxx.":   "yxxxxxxxxxxxxx+.",
			"yxxxxxxxxxxxxxxxxx...": "yxxxxxxxxxxxxx+.",
			"yxxxxxxxxxxxxxxxxx.xx": "yxxxxxxxxxxx+.xx",
			"zxxxxxx/yxxxxxxxxx":    "    z/yxxxxxxxxx",
			"zxxxxxx/yxxxxxx":       " zxxxxxx/yxxxxxx",
			"yxxxxxx":               "         yxxxxxx",
			"yxxxxxx/":              "        yxxxxxx/",
			"yxxxxxx.":              "        yxxxxxx.",
			"y.go:":                 "           y.go:",
			"y.:":                   "             y.:",
			"y.":                    "              y.",
			"y":                     "               y",
			".":                     "               .",
			".go":                   "             .go",
			"/":                     "               /",
			":":                     "               :",
			"":                      "                ",
		}
		for input, expected := range testCases {
			assert.Equal(GinkgoT(), expected, string(zapkc.FormatEntryCaller(input, 16)))
		}

		assert.Panics(GinkgoT(), func() { zapkc.FormatEntryCaller("xxxxx", 0) })
		assert.Panics(GinkgoT(), func() { zapkc.FormatEntryCaller("", 0) })
		assert.Equal(GinkgoT(), "+", string(zapkc.FormatEntryCaller("xxxxx", 1)))
		assert.Equal(GinkgoT(), "x+", string(zapkc.FormatEntryCaller("xxxxx", 2)))
		assert.Equal(GinkgoT(), "xx+", string(zapkc.FormatEntryCaller("xxxxx", 3)))
		assert.Equal(GinkgoT(), "xxx+", string(zapkc.FormatEntryCaller("xxxxx", 4)))
		assert.Equal(GinkgoT(), "xxxxx", string(zapkc.FormatEntryCaller("xxxxx", 5)))
		assert.Equal(GinkgoT(), "x", string(zapkc.FormatEntryCaller("x", 1)))
		assert.Equal(GinkgoT(), " x", string(zapkc.FormatEntryCaller("x", 2)))
		assert.Equal(GinkgoT(), "  x", string(zapkc.FormatEntryCaller("x", 3)))
		assert.Equal(GinkgoT(), "   x", string(zapkc.FormatEntryCaller("x", 4)))
		assert.Equal(GinkgoT(), "    x", string(zapkc.FormatEntryCaller("x", 5)))
		assert.Equal(GinkgoT(), " ", string(zapkc.FormatEntryCaller("", 1)))
		assert.Equal(GinkgoT(), "  ", string(zapkc.FormatEntryCaller("", 2)))
		assert.Equal(GinkgoT(), "   ", string(zapkc.FormatEntryCaller("", 3)))
		assert.Equal(GinkgoT(), "    ", string(zapkc.FormatEntryCaller("", 4)))
		assert.Equal(GinkgoT(), "     ", string(zapkc.FormatEntryCaller("", 5)))
	})
	Specify("performance", func() {
		experiment := gmeasure.NewExperiment("zapkc performance")
		for i := 0; i < 10; i++ {
			testCases := []string{
				"zxxxxxx/yxxxxxxxxxxx.go:123",
				"zxxxxxx/yxxxxxx.go:123",
			}

			for i, test := range testCases {
				func(test string) {
					experiment.MeasureDuration(fmt.Sprintf("FormatEntryCaller (%d)", i), func() {
						for ii := 0; ii < 1e5; ii++ {
							_ = zapkc.FormatEntryCaller(test, 16)
						}
					})
				}(test)
			}
			experiment.MeasureDuration("fmt.Sprintf (0)", func() {
				for i := 0; i < 1e5; i++ {
					spl := strings.Split(testCases[0], "/")
					spl2 := strings.Split(spl[1], ".")
					_ = fmt.Sprintf("%16s", fmt.Sprintf("%-8s+.%s", spl2[0], spl2[1]))
				}
			})
			experiment.MeasureDuration("fmt.Sprintf (1)", func() {
				for i := 0; i < 1e5; i++ {
					spl := strings.Split(testCases[1], "/")
					spl2 := strings.Split(spl[1], ".")
					_ = fmt.Sprintf("%16s", fmt.Sprintf("%c+.%s", spl2[0][0], spl2[1]))
				}
			})
		}
		AddReportEntry(experiment.Name, experiment)
	})
})
