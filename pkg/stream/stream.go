package stream

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/containerd/console"
	"github.com/morikuni/aec"
)

type Writers struct {
	Header io.Writer
	Footer io.Writer
}

type Renderer func(console.Console, io.Writer)

type stream struct {
	c             console.Console
	renderedLines int
	header        Renderer
	footer        Renderer
}

type LogStreamOptions struct {
	console console.Console
	height  int
	delay   time.Duration
	header  Renderer
	footer  Renderer
}

type LogStreamOption func(*LogStreamOptions)

func (o *LogStreamOptions) Apply(opts ...LogStreamOption) {
	for _, op := range opts {
		op(o)
	}
}

func WithConsole(c console.Console) LogStreamOption {
	return func(o *LogStreamOptions) {
		o.console = c
	}
}

func WithMaxHeight(height int) LogStreamOption {
	return func(o *LogStreamOptions) {
		o.height = height
	}
}

func WithDelay(delay time.Duration) LogStreamOption {
	return func(o *LogStreamOptions) {
		o.delay = delay
	}
}

func WithHeader(header Renderer) LogStreamOption {
	return func(o *LogStreamOptions) {
		o.header = header
	}
}

func WithFooter(footer Renderer) LogStreamOption {
	return func(o *LogStreamOptions) {
		o.footer = footer
	}
}

func getConsoleOrPty() console.Console {
	for _, s := range []*os.File{os.Stderr, os.Stdout, os.Stdin} {
		if c, err := console.ConsoleFromFile(s); err == nil {
			return c
		}
	}
	c, _, err := console.NewPty()
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "Note: streaming logs to a pseudo-terminal")
	return c
}

func RenderLogStream(ctx context.Context, reader io.Reader, done chan struct{}, ops ...LogStreamOption) {
	doneReading := make(chan struct{})
	options := LogStreamOptions{
		height: 5,
		delay:  time.Second / 8,
	}
	options.Apply(ops...)
	if options.console == nil {
		options.console = getConsoleOrPty()
	}
	if options.height < 0 {
		// auto-detect
		sz, err := options.console.Size()
		if err != nil {
			panic(err)
		}
		options.height = int(sz.Height - 1)
		if options.header != nil {
			options.height--
		}
		if options.footer != nil {
			options.height--
		}
	}
	// Minimum height of 1
	if options.height < 1 {
		options.height = 1
	}

	s := stream{
		c:             options.console,
		renderedLines: 0,
		header:        options.header,
		footer:        options.footer,
	}
	header := new(bytes.Buffer)
	footer := new(bytes.Buffer)
	contents := NewRingLineBuffer(options.height)

	// Stream the contents of the reader into the ring buffer.
	// This is done asynchronously so that we can render the most recent
	// contents of the buffer at each tick.
	go func() {
		defer close(doneReading)
		for ctx.Err() == nil {
			if _, err := io.Copy(contents, reader); err != nil {
				return
			}
		}
	}()

	doRender := func() {
		if ctx.Err() != nil {
			return
		}

		size, err := options.console.Size()
		if err != nil {
			panic(err)
		}

		lines := 0
		b := aec.EmptyBuilder
		for i := 0; i < s.renderedLines; i++ {
			b = b.Up(1)
		}
		fmt.Fprint(s.c, b.Column(0).ANSI)

		fmt.Fprint(s.c, aec.Hide)

		header.Reset()
		footer.Reset()
		if s.header != nil {
			s.header(s.c, header)
		}
		if s.footer != nil {
			s.footer(s.c, footer)
		}

		scan := bufio.NewScanner(header)
		for scan.Scan() {
			if _, err := io.Copy(s.c, bytes.NewReader(scan.Bytes())); err != nil {
				panic(err)
			}
			lines++
			s.c.Write([]byte{'\n'})
			break // only allow 1 line in the header
		}

		contents.Foreach(func(_ int, line []byte) {
			if len(line) > int(size.Width) {
				// Trim the line if it's too long
				line = line[:size.Width-1]
			}
			count, _ := s.c.Write(line)
			if count < int(size.Width) {
				// Pad with spaces to fill the line.
				s.c.Write(bytes.Repeat([]byte{' '}, int(size.Width)-count))
			}
			lines++
			s.c.Write([]byte{'\n'})
		})

		scan = bufio.NewScanner(footer)
		for scan.Scan() {
			if _, err := io.Copy(s.c, bytes.NewReader(scan.Bytes())); err != nil {
				panic(err)
			}
			lines++
			s.c.Write([]byte{'\n'})
			break // only allow 1 line in the footer
		}

		s.renderedLines = lines
		fmt.Fprint(s.c, aec.Show)
	}

	defer func() {
		doRender()
		close(done)
	}()

	for {
		doRender()
		select {
		case <-time.After(options.delay):
		case <-doneReading:
			return
		case <-ctx.Done():
			return
		}
	}
}
