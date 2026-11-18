//go:build windows

package tracker

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	avrtDLL                = syscall.NewLazyDLL("avrt.dll")
	avrtSetCharacteristics = avrtDLL.NewProc("AvSetMmThreadCharacteristicsW")
	avrtSetPriority        = avrtDLL.NewProc("AvSetMmThreadPriority")
	avrtRevert             = avrtDLL.NewProc("AvRevertMmThreadCharacteristics")
)

// enterAudioScheduling registers the decoder worker with Windows MMCSS. This
// gives short audio production bursts reliable scheduling under foreground
// games without raising the whole application's process priority.
func enterAudioScheduling() func() {
	runtime.LockOSThread()

	taskName, err := syscall.UTF16PtrFromString("Audio")
	if err != nil {
		runtime.UnlockOSThread()
		return func() {}
	}
	var taskIndex uint32
	handle, _, _ := avrtSetCharacteristics.Call(
		uintptr(unsafe.Pointer(taskName)),
		uintptr(unsafe.Pointer(&taskIndex)),
	)
	if handle == 0 {
		runtime.UnlockOSThread()
		return func() {}
	}

	// AVRT_PRIORITY_NORMAL: MMCSS manages the thread, but it does not run at
	// critical priority and cannot starve the UI or the rest of Windows.
	_, _, _ = avrtSetPriority.Call(handle, 0)

	return func() {
		_, _, _ = avrtRevert.Call(handle)
		runtime.UnlockOSThread()
	}
}
