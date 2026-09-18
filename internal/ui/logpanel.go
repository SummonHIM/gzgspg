package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/summonhim/gzgspg/internal/logwriter"
)

// logPanel 是一个只读的滚动日志视图。
type logPanel struct {
	writer *logwriter.Writer
	entry  *widget.Entry
	box    *container.Scroll
}

func newLogPanel(w *logwriter.Writer) *logPanel {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(joinLines(w.Lines()))
	entry.Disable()

	p := &logPanel{
		writer: w,
		entry:  entry,
		box:    container.NewVScroll(entry),
	}

	// 每来一行就追加，并切回主线程
	w.Subscribe(func(line string) {
		fyne.Do(func() {
			entry.SetText(joinLines(w.Lines()))
			p.box.ScrollToBottom()
		})
	})

	return p
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
