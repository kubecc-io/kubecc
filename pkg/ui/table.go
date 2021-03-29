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
	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type TableDataSource interface {
	Title() string
	Headers() []string
	Data() (<-chan [][]string, <-chan map[int]termui.Style)
}

type Table struct {
	*widgets.Table
	data  <-chan [][]string
	style <-chan map[int]termui.Style
	src   TableDataSource
}

func NewTable(src TableDataSource) *Table {
	data, style := src.Data()
	t := &Table{
		Table: widgets.NewTable(),
		src:   src,
		data:  data,
		style: style,
	}
	t.Title = src.Title()
	t.Table.RowStyles[0] = termui.NewStyle(termui.ColorWhite, termui.ColorClear, termui.ModifierBold)
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
		select {
		case data := <-t.data:
			if len(data) > 0 {
				t.Table.TitleStyle.Fg = termui.ColorBlue
			} else {
				t.Table.TitleStyle.Fg = termui.ColorRed
			}
			h := t.src.Headers()
			t.Rows = [][]string{}
			if h != nil {
				t.Rows = append(t.Rows, h)
			}
			t.Rows = append(t.Rows, data...)
		case style := <-t.style:
			// don't overwrite existing
			for k, v := range style {
				t.RowStyles[k] = v
			}
		}
	}
}
