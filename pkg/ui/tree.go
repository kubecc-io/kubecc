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

type TreeDataSource interface {
	Title() string
	Data() <-chan []*widgets.TreeNode
}

type Tree struct {
	*widgets.Tree
	data <-chan []*widgets.TreeNode
	src  TreeDataSource
}

func NewTree(src TreeDataSource) *Tree {
	t := &Tree{
		Tree: widgets.NewTree(),
		src:  src,
		data: src.Data(),
	}
	t.Title = src.Title()
	go t.readFromDataSource()
	return t
}

func (t *Tree) readFromDataSource() {
	for {
		data := <-t.data
		if len(data) > 0 {
			t.Tree.TitleStyle.Fg = termui.ColorBlue
		} else {
			t.Tree.TitleStyle.Fg = termui.ColorRed
		}
		t.Tree.SetNodes(data)
	}
}
