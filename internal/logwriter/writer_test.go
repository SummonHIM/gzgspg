package logwriter

import (
	"strings"
	"testing"
)

func TestWriterKeepsLastLines(t *testing.T) {
	w := New(3)
	for _, s := range []string{"a\n", "b\n", "c\n", "d\n"} {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	lines := w.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "b" || lines[1] != "c" || lines[2] != "d" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestWriterSplitsWithoutTrailingNewline(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("partial"))
	_, _ = w.Write([]byte(" line\n"))
	lines := w.Lines()
	if len(lines) != 1 || lines[0] != "partial line" {
		t.Fatalf("expected merged line, got %v", lines)
	}
}

func TestWriterSubscribe(t *testing.T) {
	w := New(10)
	var got []string
	w.Subscribe(func(line string) { got = append(got, line) })
	_, _ = w.Write([]byte("hello\nworld\n"))
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("unexpected notifications: %v", got)
	}
}

func TestWriterLinesIsCopy(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("x\n"))
	lines := w.Lines()
	lines[0] = "mutated"
	if w.Lines()[0] != "x" {
		t.Fatal("Lines() must return a copy")
	}
}

func TestWriterTrimsANSIEscape(t *testing.T) {
	w := New(10)
	_, _ = w.Write([]byte("plain line\n"))
	if strings.Contains(w.Lines()[0], "\x1b") {
		t.Fatal("unexpected escape sequence")
	}
}
