package mainpage

// nextCount is the number of the tracks the user put in the queue. They are
// shown at the top of the track list, so the indexes of the list and of the
// playlist are shifted by this number.
func (m *Model) nextCount() int {
	if m.albumListActive() {
		return 0
	}
	return len(m.manualQueue)
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
