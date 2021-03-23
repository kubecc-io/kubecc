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
	"log"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type TextBox struct {
	Paragraph *widgets.Paragraph
}

func (tb *TextBox) SetText(text string) {
	tb.Paragraph.Text = text
	ui.Render(tb.Paragraph)
}

func (tb *TextBox) Run() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	tb.Paragraph = widgets.NewParagraph()
	tb.Paragraph.Text = ""
	tb.Paragraph.WrapText = true

	termWidth, termHeight := ui.TerminalDimensions()
	tb.Paragraph.SetRect(0, 0, termWidth, termHeight)

	ui.Render(tb.Paragraph)

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q", "<C-c>":
			return
		}
	}
}
