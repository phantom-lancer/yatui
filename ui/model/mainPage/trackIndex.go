package mainpage

// nextCount is the number of the extra rows the manual queue takes at the top
// of the track list. They are shown there, so the indexes of the list and of
// the playlist are shifted by this number.
func (m *Model) nextCount() int {
	if m.albumListActive() {
		return 0
	}
	count := len(m.manualQueue)
	if m.manualPlaying != nil {
		count++
	}
	return count
}

// playlistTrackIndex returns the index of the track in the playlist that is
// shown in the track list under the given index.
func (m *Model) playlistTrackIndex(index int) int {
	return index - m.nextCount()
}

// tracklistIndex returns the index of the track in the track list.
func (m *Model) tracklistIndex(index int) int {
	return index + m.nextCount()
}
