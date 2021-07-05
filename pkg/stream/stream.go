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

type Display struct {
	Header   io.Writer
	Contents io.Writer
	Footer   io.Writer
}

type Renderer func(console.Console, Display)

type stream struct {
	c             console.Console
	renderedLines int
}

type LogStreamOptions struct {
	console console.Console
	height  int
	delay   time.Duration
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
	return c
}

func NewLogStream(ctx context.Context, renderer Renderer, ops ...LogStreamOption) {
	options := LogStreamOptions{
		height: 5,
		delay:  time.Second / 8,
	}
	options.Apply(ops...)
	if options.console == nil {
		options.console = getConsoleOrPty()
	}
	s := stream{
		c:             options.console,
		renderedLines: 0,
	}
	header := new(bytes.Buffer)
	footer := new(bytes.Buffer)
	contents := NewRingLineBuffer(options.height)
	disp := Display{
		Header:   header,
		Footer:   footer,
		Contents: contents,
	}
	for {
		lines := 0
		b := aec.EmptyBuilder
		for i := 0; i < s.renderedLines; i++ {
			b = b.Up(1)
		}
		fmt.Fprint(s.c, b.Column(0).ANSI)

		fmt.Fprint(s.c, aec.Hide)

		header.Reset()
		footer.Reset()
		renderer(s.c, disp)

		scan := bufio.NewScanner(header)
		for scan.Scan() {
			if _, err := io.Copy(s.c, bytes.NewReader(scan.Bytes())); err != nil {
				panic(err)
			}
			lines++
			s.c.Write([]byte{'\n'})
		}

		contents.Foreach(func(_ int, line []byte) {
			s.c.Write(line)
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
		}

		s.renderedLines = lines
		fmt.Fprint(s.c, aec.Show)

		select {
		case <-time.After(options.delay):
		case <-ctx.Done():
			return
		}
	}
}
