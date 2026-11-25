package mainpage

// The background workers must not touch the player: every visible change goes
// through a message, so it is applied on the UI goroutine. A worker that
// called tracker.ShowError directly raced with the player updates.
type uiErrorMsg struct {
	text string
}

type uiHideErrorMsg struct{}

// reportError shows a player error from a background goroutine.
func (m *Model) reportError(text string) {
	m.Send(uiErrorMsg{text: text})
}
