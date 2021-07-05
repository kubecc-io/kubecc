package stream_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/pkg/stream"
)

var _ = Describe("Ringlinebuffer", func() {
	When("adding lines to the buffer", func() {
		b := stream.NewRingLineBuffer(10)
		It("should keep track of the line count", func() {
			for i := 0; i < 10; i++ {
				Expect(b.LineCount()).To(Equal(i))
				fmt.Fprintln(b, string(rune('a'+i)))
				Expect(b.LineCount()).To(Equal(i + 1))
			}
			fmt.Fprintln(b, "x")
			Expect(b.LineCount()).To(Equal(b.MaxLines()))
			Expect(b.LineCount()).To(Equal(10))
		})
		It("should store the lines in the correct order", func() {
			expected := []byte{'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'x'}
			b.Foreach(func(i int, line []byte) {
				Expect([]byte{expected[i]}).To(BeEquivalentTo(line))
			})
		})
	})
})
