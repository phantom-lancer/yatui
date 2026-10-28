package tracker

import (
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Rect is a rectangular area of the rendered player view.
// All coordinates are relative to the view itself.
type Rect struct {
	X, Y, Width, Height int
}

func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

func (r Rect) Contains(x, y int) bool {
	if r.IsEmpty() {
		return false
	}
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// FractionX returns the horizontal position of x inside the rect (0..1).
func (r Rect) FractionX(x int) float64 {
	if r.IsEmpty() {
		return 0
	}
	fraction := float64(x-r.X) / float64(r.Width)
	if fraction < 0 {
		return 0
	}
	if fraction > 1 {
		return 1
	}
	return fraction
}

// Offset returns the rect moved by the given screen coordinates.
func (r Rect) Offset(x, y int) Rect {
	r.X += x
	r.Y += y
	return r
}

// Regions contains clickable areas of the player view.
// It is filled in on every render, so it always describes
// the view the user actually sees.
type Regions struct {
	PrevButton Rect
	PlayButton Rect
	NextButton Rect
	Progress   Rect
	// Volume is the whole volume indicator (icon included)
	Volume Rect
	// VolumeBar is the volume level bar itself
	VolumeBar Rect
	// EQ is the equalizer button
	EQ      Rect
	Like    Rect
	Dislike Rect
}

func (r *Regions) Offset(x, y int) {
	if r == nil {
		return
	}
	r.PrevButton = r.PrevButton.Offset(x, y)
	r.PlayButton = r.PlayButton.Offset(x, y)
	r.NextButton = r.NextButton.Offset(x, y)
	r.Progress = r.Progress.Offset(x, y)
	r.Volume = r.Volume.Offset(x, y)
	r.VolumeBar = r.VolumeBar.Offset(x, y)
	r.EQ = r.EQ.Offset(x, y)
	r.Like = r.Like.Offset(x, y)
	r.Dislike = r.Dislike.Offset(x, y)
}

// SeekTo moves the playback position to the given fraction of the track length.
func (m *Model) SeekTo(fraction float64) tea.Cmd {
	if m.player == nil || m.trackWrapper == nil {
		return nil
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	target := time.Duration(float64(m.track.DurationMs)*fraction) * time.Millisecond
	return m.Rewind(target - m.Position())
}

// SeekToStart moves the playback position to the beginning of the track
// and restarts its playtime counter.
func (m *Model) SeekToStart() tea.Cmd {
	if m.player == nil || m.trackWrapper == nil || m.trackWrapper.trackBuffer == nil {
		return nil
	}

	m.player.SetVolume(0)
	m.player.Seek(0, io.SeekStart)
	m.playtime = 0
	m.playStarted = time.Now()
	m.progressFrac = 0

	return nil
}

// SetVolumeTo sets the volume to the given fraction of the maximum.
func (m *Model) SetVolumeTo(fraction float64) {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	m.SetVolume(fraction)
}

// NudgeVolume increases (or decreases, for a negative delta) the volume.
func (m *Model) NudgeVolume(delta float64) {
	m.SetVolume(m.Volume() + delta)
}
