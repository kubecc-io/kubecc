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
	tb.Paragraph.WrapText = false

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
