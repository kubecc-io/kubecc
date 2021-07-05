package stream_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/andreyvit/diff"
	"github.com/containerd/console"
	"github.com/kubecc-io/kubecc/pkg/stream"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		read, write := io.Pipe()

		ctx, ca := context.WithCancel(context.Background())
		done := make(chan struct{})
		stream.RenderLogStream(ctx, read, done, stream.WithConsole(pty), stream.WithHeader(func(c console.Console, w io.Writer) {
			if i == 20 {
				defer ca()
			}
			Expect(i).To(BeNumerically("<=", 20))
			sz, _ := c.Size()
			fmt.Fprintln(w, strings.Repeat("H", int(sz.Width)))
			if i > 0 {
				fmt.Fprintln(write, strings.Repeat(string(rune('a'+i-1)), int(sz.Width)))
			}
			i++
		}), stream.WithFooter(func(c console.Console, w io.Writer) {
			sz, _ := c.Size()
			fmt.Fprintln(w, strings.Repeat("F", int(sz.Width)))
		}))

		sz, err := pty.Size()
		Expect(err).NotTo(HaveOccurred())

		sampleOut := sampleOutput(sz)
		buf := make([]byte, len(sampleOut))
		_, err = pty.Read(buf)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(string(buf))).To(
			BeEquivalentTo(strings.TrimSpace(string(sampleOut))),
			diff.LineDiff(strings.TrimSpace(string(buf)), strings.TrimSpace(string(sampleOut))))
	})
})
