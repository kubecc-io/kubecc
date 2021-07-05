package stream

import (
	"bytes"
	"container/ring"
	"sync"
)

/*

Max: 4

Items listed in the order they appear in Foreach() (nil items are skipped)

0 items
* nil <- first, last
* nil
* nil
* nil

1 item
* "one" <- first
* nil <- last
* nil
* nil

2 items
* "one" <- first
* "two"
* nil <- last
* nil

3 items
* "one" <- first
* "two"
* "three"
* nil <- last

4 items
* "one" <- first, last
* "two"
* "three"
* "four"

5 items
* "two" <- first, last
* "three"
* "four"
* "five"


*/

type RingLineBuffer struct {
	maxLines int
	curLines int
	first    *ring.Ring // []byte{}
	last     *ring.Ring // []byte{}
	buf      *bytes.Buffer
	lock     sync.Mutex
}

func NewRingLineBuffer(max int) *RingLineBuffer {
	wb := &RingLineBuffer{
		maxLines: max,
		curLines: 0,
		first:    ring.New(max),
		buf:      new(bytes.Buffer),
	}
	wb.last = wb.first
	for i := 0; i < max; i++ {
		wb.first.Move(i).Value = make([]byte, 0)
	}
	return wb
}

func (wb *RingLineBuffer) addLine(line []byte) {
	wb.last.Value = line
	wb.last = wb.last.Next()
	if wb.curLines < wb.maxLines {
		wb.curLines++
	} else {
		wb.first = wb.last
	}
}

func (wb *RingLineBuffer) Write(data []byte) (int, error) {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	// Write to buffer
	for _, b := range data {
		if b == '\n' {
			// Ensure the contents of the buffer are copied
			wb.addLine(append([]byte(nil), wb.buf.Bytes()...))
			wb.buf.Reset()
			// skip the \n
		} else {
			wb.buf.WriteByte(b)
		}
	}
	return len(data), nil
}

func (wb *RingLineBuffer) LineCount() int {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	return wb.curLines
}

func (wb *RingLineBuffer) MaxLines() int {
	return wb.maxLines
}

func (wb *RingLineBuffer) Foreach(f func(i int, line []byte)) {
	wb.lock.Lock()
	defer wb.lock.Unlock()

	cur := wb.first
	for i := 0; i < wb.curLines; i++ {
		f(i, cur.Value.([]byte))
		cur = cur.Next()
	}
}
