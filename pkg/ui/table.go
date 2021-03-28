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

package ui

import (
	"github.com/gizak/termui/v3/widgets"
)

type TableDataSource interface {
	Title() string
	Headers() []string
	Data() <-chan [][]string
}

type Table struct {
	*widgets.Table
	data <-chan [][]string
	src  TableDataSource
}

func NewTable(src TableDataSource) *Table {
	t := &Table{
		Table: widgets.NewTable(),
		src:   src,
		data:  src.Data(),
	}
	t.Title = src.Title()
	go t.readFromDataSource()
	return t
}

func (t *Table) readFromDataSource() {
	h := t.src.Headers()
	t.Rows = [][]string{}
	if h != nil {
		t.Rows = append(t.Rows, h)
	}
	for {
		data := <-t.data
		h := t.src.Headers()
		t.Rows = [][]string{}
		if h != nil {
			t.Rows = append(t.Rows, h)
		}
		t.Rows = append(t.Rows, data...)
	}
}
