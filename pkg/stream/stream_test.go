package stream_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/containerd/console"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kubecc-io/kubecc/pkg/stream"
)

func sampleOutput(sz console.WinSize) []byte {
	expectedLines := [][]rune{
		{},
		{'a'},
		{'a', 'b'},
		{'a', 'b', 'c'},
		{'a', 'b', 'c', 'd'},
		{'a', 'b', 'c', 'd', 'e'},
		{'b', 'c', 'd', 'e', 'f'},
		{'c', 'd', 'e', 'f', 'g'},
		{'d', 'e', 'f', 'g', 'h'},
		{'e', 'f', 'g', 'h', 'i'},
		{'f', 'g', 'h', 'i', 'j'},
		{'g', 'h', 'i', 'j', 'k'},
		{'h', 'i', 'j', 'k', 'l'},
		{'i', 'j', 'k', 'l', 'm'},
		{'j', 'k', 'l', 'm', 'n'},
		{'k', 'l', 'm', 'n', 'o'},
		{'l', 'm', 'n', 'o', 'p'},
		{'m', 'n', 'o', 'p', 'q'},
		{'n', 'o', 'p', 'q', 'r'},
		{'o', 'p', 'q', 'r', 's'},
		{'p', 'q', 'r', 's', 't'},
	}
	buf := new(bytes.Buffer)
	for i := 0; i < 20; i++ {
		if i > 0 {
			buf.WriteString(`^[[?25h` + strings.Repeat(`^[[1A`, int(math.Min(float64(i+1), 7))))
		}
		buf.WriteString(`^[[0G^[[?25l`)
		buf.WriteString(strings.Repeat("H", int(sz.Width)) + "\r\n") // the PTY uses CRLF
		for ii := 0; ii < int(math.Min(float64(i), 5)); ii++ {
			buf.WriteString(strings.Repeat(string(expectedLines[i][ii]), int(sz.Width)) + "\r\n")
		}
		buf.WriteString(strings.Repeat("F", int(sz.Width)) + "\r\n")
	}
	buf.WriteString(`^[[?25h`)
	return buf.Bytes()
}

var _ = Describe("Stream", func() {
	Specify("sample test", func() {
		i := 0
		pty, _, err := console.NewPty()
		pty.Resize(console.WinSize{
			Height: 500,
			Width:  5,
		})
		Expect(err).NotTo(HaveOccurred())
		ctx, ca := context.WithCancel(context.Background())
		stream.NewLogStream(ctx, func(c console.Console, d stream.Display) {
			if i == 20 {
				defer ca()
			}
			Expect(i).To(BeNumerically("<=", 20))
			sz, _ := c.Size()
			fmt.Fprintln(d.Header, strings.Repeat("H", int(sz.Width)))
			if i > 0 {
				fmt.Fprintln(d.Contents, strings.Repeat(string(rune('a'+i-1)), int(sz.Width)))
			}
			fmt.Fprintln(d.Footer, strings.Repeat("F", int(sz.Width)))
			i++
		}, stream.WithConsole(pty))
		sz, err := pty.Size()
		Expect(err).NotTo(HaveOccurred())

		sampleOut := sampleOutput(sz)
		buf := make([]byte, len(sampleOut))
		_, err = pty.Read(buf)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes.TrimSpace(buf)).To(
			BeEquivalentTo(bytes.TrimSpace(sampleOut)))
	})
})
