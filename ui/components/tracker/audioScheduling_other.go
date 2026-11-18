//go:build !windows

package tracker

func enterAudioScheduling() func() {
	return func() {}
}
