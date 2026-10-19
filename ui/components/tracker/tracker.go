package tracker

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/config"
	"github.com/dece2183/yamusic-tui/log"
	"github.com/dece2183/yamusic-tui/stream"
	"github.com/dece2183/yamusic-tui/ui/helpers"
	"github.com/dece2183/yamusic-tui/ui/model"
	"github.com/dece2183/yamusic-tui/ui/style"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ebitengine/oto/v3"
)

type Control uint

const (
	PLAY Control = iota
	PAUSE
	STOP
	NEXT
	PREV
	LIKE
	DISLIKE
	REWIND
	VOLUME
	CACHE_TRACK
	BUFFERING_COMPLETE
	TOGGLE_LYRICS
	TOGGLE_VIEW
)

type ProgressControl float64

func (p ProgressControl) Value() float64 {
	return float64(p)
}

const (
	_VOLUME_FADE_STEPS     = 2
	_VOLUME_SNAP_THRESHOLD = 0.005 // snap to 0/1 when close enough
	_VOLUME_FADE_PERIOD    = 60 * time.Millisecond
)

type Model struct {
	width      int
	track      api.Track
	lyrics     []api.LyricPair
	progress   progress.Model
	volumeBar  progress.Model
	help       help.Model
	helpMap    *helpKeyMap
	Hidden     bool
	showLyrics bool
	showError  bool
	errorText  string

	paused         bool
	playtime       time.Duration
	playStarted    time.Time
	volume         float64
	volumeIncremet float64
	lastVolumeKey  time.Time
	rewindAmount   time.Duration
	playerContext  *oto.Context
	player         *oto.Player
	trackWrapper   *readWrapper

	program     *tea.Program
	likesMap    *map[string]bool
	dislikesMap *map[string]bool

	// nextTitle is the track that will be played after the current one.
	nextTitle string

	regions *Regions
}

func New(p *tea.Program, likesMap *map[string]bool, dislikesMap *map[string]bool) *Model {
	m := &Model{
		program:     p,
		likesMap:    likesMap,
		dislikesMap: dislikesMap,
		progress:    progress.New(),
		volumeBar:   progress.New(),
		help:        help.New(),
		helpMap:     newHelpMap(),
		paused:      true,
		volume:      config.Current.Volume,
		showLyrics:  config.Current.ShowLyrics,
	}

	m.volumeIncremet = m.volume / _VOLUME_FADE_STEPS
	m.rewindAmount = time.Duration(config.Current.RewindDuration) * time.Second

	m.progress.ShowPercentage = false
	m.progress.Empty = m.progress.Full
	m.progress.FullColor = string(style.AccentColor)
	m.progress.EmptyColor = string(style.BackgroundColor)
	m.progress.SetSpringOptions(60, 1)

	m.volumeBar.FullColor = string(style.AccentColor)
	m.volumeBar.EmptyColor = string(style.BackgroundColor)
	m.volumeBar.Width = style.VolumeIndicatorWidth

	m.help.Ellipsis = "…"
	m.trackWrapper = &readWrapper{program: m.program}

	op := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		BufferSize:   time.Millisecond * time.Duration(config.Current.BufferSize),
		Format:       oto.FormatSignedInt16LE,
	}

	var err error
	var readyChan chan struct{}
	m.playerContext, readyChan, err = oto.NewContext(op)
	if err != nil {
		log.Print(log.LVL_PANIC, "failed to create player context: %s", err)
		model.PrettyExit(err, 12)
	}
	<-readyChan

	go m.volumeFader()
	return m
}

// RefreshStyle re-reads the colors from the style package. The progress and
// volume bars keep their colors inside the component, so a theme switch has
// to push them again.
func (m *Model) RefreshStyle() {
	m.progress.FullColor = string(style.AccentColor)
	m.progress.EmptyColor = string(style.BackgroundColor)
	m.volumeBar.FullColor = string(style.AccentColor)
	m.volumeBar.EmptyColor = string(style.BackgroundColor)
}

// SetNextTrack sets the "next:" line under the current track. An empty title
// hides the line.
func (m *Model) SetNextTrack(title string) {
	m.nextTitle = title
}

// nextTrackLine renders the upcoming track hint, truncated to maxLen cells.
func (m *Model) nextTrackLine(maxLen int) string {
	if m.nextTitle == "" || maxLen <= 0 {
		return ""
	}

	line := style.TrackVersionStyle.Render("Далее: " + m.nextTitle)
	if lipgloss.Width(line) > maxLen {
		line = lipgloss.NewStyle().MaxWidth(maxLen-1).Render(line) + "…"
	}
	return line
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	view, _ := m.view()
	return view
}

// view renders the player and reports clickable areas of the rendered view.
func (m *Model) view() (string, *Regions) {
	if m.Hidden {
		m.regions = nil
		return "", nil
	}

	regions := &Regions{}

	// box borders and padding offsets of the view content
	contentX := style.TrackBoxStyle.GetBorderLeftSize() + style.TrackBoxStyle.GetPaddingLeft()
	contentY := style.TrackBoxStyle.GetBorderTopSize() + style.TrackBoxStyle.GetPaddingTop()

	// transport buttons: prev / play / next, spaced apart so that they
	// stay readable and easy to hit
	var playButton string
	if m.IsPlaying() {
		playButton = style.ActiveButtonStyle.Padding(0, 1).Margin(0).Render(style.IconPlay)
	} else {
		playButton = style.ActiveButtonStyle.Padding(0, 1).Margin(0).Render(style.IconStop)
	}

	prevButton := style.ButtonStyle.Padding(0, 1).Margin(0).Render(style.IconPrev)
	nextButton := style.ButtonStyle.Padding(0, 1).Margin(0).Render(style.IconNext)
	transportGap := "  "

	var volumeIndicator string
	var volumeIndicatorWidth int
	var volumeBarX int
	if style.VolumeIndicatorWidth > 0 && m.width > style.VolumeIndicatorAutohide {
		var volumeIcon string
		if m.volume <= 0 {
			volumeIcon = style.IconVolumeOff
		} else if m.volume < 0.1 {
			volumeIcon = style.IconVolumeLow
		} else if m.volume < 0.5 {
			volumeIcon = style.IconVolumeMid
		} else {
			volumeIcon = style.IconVolumeHigh
		}
		volumeIndicator = " " + volumeIcon + " " + m.volumeBar.ViewAs(m.volume)
		volumeIndicatorWidth = lipgloss.Width(volumeIndicator)
		volumeBarX = 1 + lipgloss.Width(volumeIcon) + 1
	}

	// the progress bar must fit between the controls and the volume indicator
	fixedWidth := contentX +
		lipgloss.Width(prevButton) +
		len(transportGap) +
		lipgloss.Width(playButton) +
		len(transportGap) +
		lipgloss.Width(nextButton) +
		len(transportGap) +
		style.TrackProgressStyle.GetPaddingLeft() +
		volumeIndicatorWidth +
		1
	m.progress.Width = m.width - fixedWidth
	if m.progress.Width < 0 {
		m.progress.Width = 0
	}
	progressBlock := style.TrackProgressStyle.Render(m.progress.View())
	tracker := lipgloss.JoinHorizontal(
		lipgloss.Top,
		prevButton, transportGap, playButton, transportGap, nextButton, transportGap,
		progressBlock, volumeIndicator,
	)

	topOffset := 0
	if m.showLyrics {
		lyrics := m.renderLyrics()
		topOffset += lipgloss.Height(lyrics) + 1
		tracker = lipgloss.JoinVertical(lipgloss.Left, lyrics, "", tracker)
	}

	if m.showError && !config.Current.SuppressErrors {
		errText := "Error: " + m.errorText + "; -> " + log.Location()
		maxLen := m.Width() - 4
		if lipgloss.Width(errText) > maxLen {
			errText = lipgloss.NewStyle().MaxWidth(maxLen-1).Render(errText) + "…"
		}
		topOffset += 2
		tracker = lipgloss.JoinVertical(lipgloss.Left, style.ErrorTextStyle.Render(errText), "", tracker)
	}

	// progress bar row regions
	rowY := contentY + topOffset
	prevW := lipgloss.Width(prevButton)
	playW := lipgloss.Width(playButton)
	nextW := lipgloss.Width(nextButton)
	gapW := len(transportGap)

	prevX := contentX
	playX := prevX + prevW + gapW
	nextX := playX + playW + gapW
	progressX := nextX + nextW + gapW + style.TrackProgressStyle.GetPaddingLeft()
	volumeX := progressX - style.TrackProgressStyle.GetPaddingLeft() + lipgloss.Width(progressBlock)

	regions.PrevButton = Rect{X: prevX, Y: rowY, Width: prevW, Height: 1}
	regions.PlayButton = Rect{X: playX, Y: rowY, Width: playW, Height: 1}
	regions.NextButton = Rect{X: nextX, Y: rowY, Width: nextW, Height: 1}
	regions.Progress = Rect{X: progressX, Y: rowY, Width: m.progress.Width, Height: 1}
	regions.Volume = Rect{X: volumeX, Y: rowY, Width: volumeIndicatorWidth, Height: 1}
	regions.VolumeBar = Rect{X: volumeX + volumeBarX, Y: rowY, Width: m.volumeBar.Width, Height: 1}
	regions.Dislike = Rect{X: 0, Y: 0, Width: 0, Height: 0}

	titleRowY := rowY + lipgloss.Height(progressBlock)

	var trackTitle string
	if !m.help.ShowAll {
		disliked := m.dislikesMap != nil && (*m.dislikesMap)[m.track.Id]

		// unavailable and disliked tracks are greyed out
		titleStyle := style.TrackTitleStyle
		artistStyle := style.TrackArtistStyle
		if !m.track.Available || disliked {
			titleStyle = titleStyle.Foreground(style.InactiveTextColor)
			artistStyle = artistStyle.Foreground(style.InactiveTextColor)
		}
		if !m.track.Available || disliked {
			trackTitle = titleStyle.Strikethrough(true).Render(m.track.Title)
		} else {
			trackTitle = titleStyle.Render(m.track.Title)
		}

		trackVersion := style.TrackVersionStyle.Render(" " + m.track.Version)
		trackTitle = lipgloss.JoinHorizontal(lipgloss.Top, trackTitle, trackVersion)

		durTotal := time.Millisecond * time.Duration(m.track.DurationMs)
		durEllapsed := time.Millisecond * time.Duration(float64(m.track.DurationMs)*m.progress.Percent())
		trackTime := style.TrackVersionStyle.Render(fmt.Sprintf("%02d:%02d/%02d:%02d",
			int(durEllapsed.Minutes()),
			int(durEllapsed.Seconds())%60,
			int(durTotal.Minutes()),
			int(durTotal.Seconds())%60,
		))

		var trackLike string
		if (*m.likesMap)[m.track.Id] {
			trackLike = style.IconLiked + " "
		} else {
			trackLike = style.IconNotLiked + " "
		}

		// the dislike indicator sits next to the like one
		trackDislike := style.TrackAddInfoStyle.Render(style.IconDisliked + " ")

		trackAddInfo := style.TrackAddInfoStyle.Render(trackLike + trackDislike + trackTime)
		addInfoLen := lipgloss.Width(trackAddInfo)
		maxLen := m.Width() - addInfoLen - 4
		stl := lipgloss.NewStyle().MaxWidth(maxLen - 1)

		trackTitleLen := lipgloss.Width(trackTitle)
		if trackTitleLen > maxLen {
			trackTitle = stl.Render(trackTitle) + "…"
		} else if trackTitleLen < maxLen {
			trackTitle += strings.Repeat(" ", maxLen-trackTitleLen)
		}

		trackArtist := artistStyle.Render(helpers.ArtistList(m.track.Artists))
		trackArtistLen := lipgloss.Width(trackArtist)
		if trackArtistLen > maxLen {
			trackArtist = stl.Render(trackArtist) + "…"
		} else if trackArtistLen < maxLen {
			trackArtist += strings.Repeat(" ", maxLen-trackArtistLen)
		}

		trackTitle = lipgloss.NewStyle().Width(m.width - lipgloss.Width(trackAddInfo) - 4).Render(trackTitle)
		trackTitle = lipgloss.JoinHorizontal(lipgloss.Top, trackTitle, trackAddInfo)

		// the "next" line is always reserved, so the player height (and the
		// track list above it) does not jump when the queue ends
		rows := []string{trackTitle, trackArtist, m.nextTrackLine(maxLen), ""}
		trackTitle = lipgloss.JoinVertical(lipgloss.Left, rows...)

		// like / dislike indicator regions on the track title row
		addInfoX := m.width - addInfoLen - 4
		if addInfoX < 0 {
			addInfoX = 0
		}

		regions.Like = Rect{
			X:      contentX + addInfoX,
			Y:      titleRowY,
			Width:  lipgloss.Width(trackLike),
			Height: 1,
		}
		regions.Dislike = Rect{
			X:      contentX + addInfoX + lipgloss.Width(trackLike),
			Y:      titleRowY,
			Width:  lipgloss.Width(trackDislike),
			Height: 1,
		}

		tracker = lipgloss.JoinVertical(lipgloss.Left, tracker, trackTitle)
	}

	tracker = lipgloss.JoinVertical(lipgloss.Left, tracker, m.help.View(m.helpMap))
	m.regions = regions
	return style.TrackBoxStyle.Width(m.width).Render(tracker), regions
}

// Regions returns clickable areas of the last rendered view.
func (m *Model) Regions() *Regions {
	return m.regions
}

func (m *Model) Update(message tea.Msg) (*Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := message.(type) {
	case tea.KeyMsg:
		controls := config.Current.Controls
		keypress := msg.String()

		switch {
		case controls.ShowAllKeys.Contains(keypress):
			m.help.ShowAll = !m.help.ShowAll

		case controls.PlayerPause.Contains(keypress), controls.PlayerStop.Contains(keypress):
			if m.player == nil {
				break
			}
			if m.player.IsPlaying() {
				m.Pause()
				cmds = append(cmds, model.Cmd(PAUSE))
			} else {
				m.Play()
				cmds = append(cmds, model.Cmd(PLAY))
			}

		case controls.PlayerRewindForward.Contains(keypress):
			cmds = append(cmds, model.Cmd(NEXT))

		case controls.PlayerRewindBackward.Contains(keypress):
			cmd = m.Rewind(-m.rewindAmount)
			cmds = append(cmds, cmd, model.Cmd(REWIND))

		case controls.PlayerNext.Contains(keypress):
			cmds = append(cmds, model.Cmd(NEXT))

		case controls.PlayerPrevious.Contains(keypress):
			cmds = append(cmds, model.Cmd(PREV))

		case controls.PlayerLike.Contains(keypress):
			cmds = append(cmds, model.Cmd(LIKE))

		case controls.PlayerCache.Contains(keypress):
			if !m.IsStoped() {
				m.trackWrapper.trackBuffer.BufferAll()
				cmds = append(cmds, model.Cmd(CACHE_TRACK))
			}

		case controls.PlayerVolUp.Contains(keypress):
			m.SetVolume(m.volume + m.dynamicVolumeStep())
			cmds = append(cmds, model.Cmd(VOLUME))

		case controls.PlayerVolDown.Contains(keypress):
			m.SetVolume(m.volume - m.dynamicVolumeStep())
			cmds = append(cmds, model.Cmd(VOLUME))

		case controls.PlayerToggleLyrics.Contains(keypress):
			m.SetLirycs(!m.showLyrics)
			cmds = append(cmds, model.Cmd(TOGGLE_LYRICS))
		case controls.PlayerHide.Contains(keypress):
			m.Hidden = !m.Hidden
			cmds = append(cmds, model.Cmd(TOGGLE_VIEW))
		}

	// player control update
	case Control:
		switch msg {
		case PLAY:
			m.Play()
		case PAUSE:
			m.Pause()
		case STOP:
			m.Stop()
		}

	// track progress update
	case ProgressControl:
		cmd = m.progress.SetPercent(msg.Value())
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		var progressModel tea.Model
		progressModel, cmd = m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) SetWidth(width int) {
	m.width = width
	m.help.Width = width - 4
}

func (m *Model) Width() int {
	return m.width
}

func (m *Model) Height() int {
	baseHeight := 4
	if m.showLyrics {
		baseHeight += 4
	}
	if m.showError && !config.Current.SuppressErrors {
		baseHeight += 2
	}
	return baseHeight
}

func (m *Model) Progress() float64 {
	return m.progress.Percent()
}

func (m *Model) Position() time.Duration {
	return time.Duration(float64(m.track.DurationMs)*m.trackWrapper.Progress()) * time.Millisecond
}

func (m *Model) SetVolume(v float64) {
	if v < _VOLUME_SNAP_THRESHOLD {
		v = 0
	} else if v > 1-_VOLUME_SNAP_THRESHOLD {
		v = 1
	}
	m.volume = v
	m.volumeIncremet = m.volume / _VOLUME_FADE_STEPS
	config.Current.Volume = m.volume
	config.Save()
}

// LastGain reports the correction, in decibels, that the loudness
// normalization applied to the current track. It is 0 when the track is
// played as is or when nothing has been played yet.
func (m *Model) LastGain() float64 {
	if m.trackWrapper == nil {
		return 0
	}
	return math.Float64frombits(atomic.LoadUint64(&m.trackWrapper.gainDB))
}

func (m *Model) SetLirycs(show bool) {
	m.showLyrics = show
	config.Current.ShowLyrics = m.showLyrics
	config.Save()
}

// SetRewind changes the rewind step for the skip keys.
func (m *Model) SetRewind(amount time.Duration) {
	if amount < time.Second {
		amount = time.Second
	}
	m.rewindAmount = amount
	config.Current.RewindDuration = amount.Seconds()
	config.Save()
}

// Lyrics reports whether the lyrics panel is shown.
func (m *Model) Lyrics() bool {
	return m.showLyrics
}

func (m *Model) Volume() float64 {
	return m.volume
}

func (m *Model) StartTrack(track *api.Track, reader *stream.BufferedStream, lyrics []api.LyricPair) {
	m.showError = false
	m.volume = config.Current.Volume
	m.volumeIncremet = m.volume / _VOLUME_FADE_STEPS

	if m.player != nil {
		m.Stop()
	}

	m.track = *track
	m.trackWrapper.NewReader(reader, &m.track)
	m.player = m.playerContext.NewPlayer(m.trackWrapper)
	m.player.SetVolume(0)
	m.player.Play()
	m.lyrics = lyrics
	m.paused = false
	m.playtime = 0
	m.playStarted = time.Now()
}

func (m *Model) Stop() {
	if m.player == nil {
		return
	}

	if m.player.IsPlaying() {
		m.player.SetVolume(0)
		m.player.Pause()
	}

	if m.trackWrapper.trackBuffer.Error() != nil {
		m.ShowError("track buffering")
	}

	m.trackWrapper.Close()
	m.player.Close()
	m.player = nil
	m.playtime += time.Since(m.playStarted)
	m.paused = true
}

func (m *Model) IsPlaying() bool {
	return m.player != nil && m.trackWrapper.trackBuffer != nil && m.player.IsPlaying() && !m.paused
}

// Paused reports whether the playback is paused.
func (m *Model) Paused() bool {
	return m.paused
}

func (m *Model) IsStoped() bool {
	return m.player == nil || m.trackWrapper.trackBuffer == nil
}

func (m *Model) CurrentTrack() *api.Track {
	return &m.track
}

func (m *Model) Play() {
	if m.player == nil || m.trackWrapper.trackBuffer == nil {
		return
	}
	if m.player.IsPlaying() {
		return
	}
	m.volume = config.Current.Volume
	m.volumeIncremet = m.volume / _VOLUME_FADE_STEPS
	m.player.SetVolume(0)
	m.player.Play()
	m.paused = false
	m.playStarted = time.Now()
}

func (m *Model) Pause() {
	if m.player == nil || m.trackWrapper.trackBuffer == nil {
		return
	}
	if !m.player.IsPlaying() {
		return
	}
	m.playtime += time.Since(m.playStarted)
	m.paused = true
}

func (m *Model) Rewind(amount time.Duration) tea.Cmd {
	if m.player == nil || m.trackWrapper == nil {
		go m.program.Send(STOP)
		return nil
	}

	m.player.SetVolume(0)

	amountMs := float64(amount.Milliseconds())
	currentPos := int64(float64(m.trackWrapper.Length()) * m.trackWrapper.Progress())
	byteOffset := int64(math.Round((float64(m.trackWrapper.Length()) / float64(m.track.DurationMs)) * amountMs))

	// align position by 4 bytes
	currentPos += byteOffset
	currentPos += currentPos % 4

	if currentPos <= 0 {
		m.player.Seek(0, io.SeekStart)
	} else if currentPos >= m.trackWrapper.Length() {
		m.player.Seek(0, io.SeekEnd)
	} else {
		m.player.Seek(currentPos, io.SeekStart)
	}

	return m.progress.SetPercent(m.trackWrapper.Progress())
}

func (m *Model) SetPos(pos time.Duration) {
	if m.player == nil || m.trackWrapper == nil {
		go m.program.Send(STOP)
		return
	}

	posMs := pos.Milliseconds()
	byteOffset := int64(math.Round((float64(m.trackWrapper.Length()) / float64(m.track.DurationMs)) * float64(posMs)))

	// align position by 4 bytes
	byteOffset += byteOffset % 4
	m.player.Seek(byteOffset, io.SeekStart)
}

func (m *Model) TrackBuffer() *stream.BufferedStream {
	return m.trackWrapper.trackBuffer
}

func (m *Model) Playtime() time.Duration {
	if m.paused {
		return m.playtime
	}
	return m.playtime + time.Since(m.playStarted)
}

func (m *Model) ShowError(text string) {
	m.showError = true
	m.errorText = text
}

func (m *Model) HideError() {
	m.showError = false
}

func (m *Model) dynamicVolumeStep() float64 {
	now := time.Now()
	delta := now.Sub(m.lastVolumeKey)
	m.lastVolumeKey = now

	step := config.Current.VolumeStep
	switch {
	case delta < 80*time.Millisecond:
		return step
	case delta < 200*time.Millisecond:
		return step * 0.4
	default:
		return step * 0.2
	}
}

func (m *Model) volumeFadeTick() {
	if m.player == nil || !m.player.IsPlaying() {
		return
	}

	if m.volumeIncremet == 0 {
		m.player.SetVolume(0)
		return
	}

	var targetVolume float64
	if m.paused {
		targetVolume = 0
	} else {
		targetVolume = m.volume
	}

	currVol := m.player.Volume()
	if currVol >= targetVolume+m.volumeIncremet {
		m.player.SetVolume(currVol - m.volumeIncremet/2)
	} else if currVol <= targetVolume-m.volumeIncremet {
		m.player.SetVolume(currVol + m.volumeIncremet/2)
	} else if currVol != targetVolume {
		m.player.SetVolume(targetVolume)
		if m.paused {
			m.player.Pause()
		}
	}
}

func (m *Model) volumeFader() {
	t := time.NewTicker(_VOLUME_FADE_PERIOD)
	for range t.C {
		m.volumeFadeTick()
	}
}

func (m *Model) renderLyrics() string {
	currentLine := " "
	nextLine := " "
	previousLine := " "

	if m.player != nil && m.showLyrics {
		switch m.track.LyricsInfo.HasAvailableSyncLyrics {
		case true:
			for idx, line := range m.lyrics {
				if line.Timestamp > int(m.Position().Milliseconds()-1000) {
					previousLine = m.tryGetLyricsLine(idx - 2)
					currentLine = m.lyricsBreak(m.tryGetLyricsLine(idx - 1))
					nextLine = m.tryGetLyricsLine(idx)
					break
				}
			}
		case false:
			currentLine = "This song doesn't have synced lyrics!"
		}
	}

	previousLine = lipgloss.NewStyle().Foreground(style.LyricsPreviosTextColor).Render(previousLine)
	nextLine = lipgloss.NewStyle().Foreground(style.LyricsNextTextColor).Render(nextLine)
	currentLine = lipgloss.NewStyle().Foreground(style.LyricsCurrentTextColor).Render(currentLine)

	lyrics := lipgloss.JoinVertical(lipgloss.Center, previousLine, currentLine, nextLine)
	lyrics = lipgloss.NewStyle().Width(m.width - 4).AlignHorizontal(lipgloss.Center).Render(lyrics)

	return lyrics
}

func (m *Model) tryGetLyricsLine(idx int) (line string) {
	if idx < 0 || idx >= len(m.lyrics) {
		return
	}
	return m.lyrics[idx].Line
}

func (m *Model) lyricsBreak(line string) (newLine string) {
	if strings.TrimSpace(strings.TrimSpace(line)) != "" {
		return line
	}

	switch m.Position().Milliseconds() % 900 / 300 {
	default:
		newLine = style.IconDotLight + style.IconDotDark + style.IconDotDark
	case 1:
		newLine = style.IconDotDark + style.IconDotLight + style.IconDotDark
	case 2:
		newLine = style.IconDotDark + style.IconDotDark + style.IconDotLight
	}

	return
}
