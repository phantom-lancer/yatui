package mainpage

import (
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/dece2183/yamusic-tui/ui/style"
)

const (
	toastDuration = 2600 * time.Millisecond
	toastMargin   = 1
	maxToasts     = 3
)

type toast struct {
	text    string
	expires time.Time
}

// toastExpiredMsg is sent by a one-shot timer when the oldest toast has to be
// dropped. There is no permanent ticker: in idle nothing is scheduled.
type toastExpiredMsg struct{}

// showToast displays a short status message above the bottom edge of the
// screen. It is safe to call from any handler.
func (m *Model) showToast(text string) {
	if text == "" {
		return
	}

	m.toasts = append(m.toasts, toast{text: text, expires: time.Now().Add(toastDuration)})
	if len(m.toasts) > maxToasts {
		m.toasts = m.toasts[len(m.toasts)-maxToasts:]
	}

	if m.program == nil {
		return
	}
	time.AfterFunc(toastDuration+50*time.Millisecond, func() {
		m.program.Send(toastExpiredMsg{})
	})
}

func (m *Model) expireToasts() {
	now := time.Now()
	alive := m.toasts[:0]
	for _, t := range m.toasts {
		if t.expires.After(now) {
			alive = append(alive, t)
		}
	}
	m.toasts = alive
}

// drawToasts writes the toasts over the bottom right corner of the view.
func (m *Model) drawToasts(view string) string {
	if len(m.toasts) == 0 {
		return view
	}

	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		return view
	}

	// the last line is usually empty padding, so start from the last
	// line that actually has something on it
	last := len(lines) - 1
	for last > 0 && strings.TrimSpace(ansi.Strip(lines[last])) == "" {
		last--
	}

	for i, t := range m.toasts {
		box := style.ToastAccentStyle.Render("▌") + style.ToastStyle.Render(" "+t.text)
		y := last - (len(m.toasts) - 1 - i)
		if y < 0 {
			break
		}

		lineWidth := ansi.StringWidth(lines[y])
		x := lineWidth - ansi.StringWidth(box) - toastMargin
		if x < 0 {
			x = 0
		}
		lines[y] = overlay(lines[y], box, x)
	}

	return strings.Join(lines, "\n")
}
