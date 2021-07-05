package stream

import (
	"bufio"
	"bytes"
	"container/ring"
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
}

func NewRingLineBuffer(max int) *RingLineBuffer {
	wb := &RingLineBuffer{
		maxLines: max,
		curLines: 0,
		first:    ring.New(max),
	}
	wb.last = wb.first
	for i := 0; i < max; i++ {
		wb.first.Move(i).Value = make([]byte, 0)
	}
	return wb
}

func (wb *RingLineBuffer) writeLine(data []byte) int {
	wb.last.Value = data
	wb.last = wb.last.Next()
	if wb.curLines < wb.maxLines {
		wb.curLines++
	} else {
		wb.first = wb.last
	}
	return len(data)
}

func (wb *RingLineBuffer) Write(data []byte) (int, error) {
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Split(bufio.ScanLines)
	total := 0
	for scan.Scan() {
		total += wb.writeLine(scan.Bytes())
	}
	return total, nil
}

func (wb *RingLineBuffer) LineCount() int {
	return wb.curLines
}

func (wb *RingLineBuffer) MaxLines() int {
	return wb.maxLines
}

func (wb *RingLineBuffer) Foreach(f func(i int, line []byte)) {
	cur := wb.first
	for i := 0; i < wb.LineCount(); i++ {
		f(i, cur.Value.([]byte))
		cur = cur.Next()
	}
}
