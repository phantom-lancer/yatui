package stream

import (
	"errors"
	"io"
	"sync"
	"time"
)

const (
	_BUFFERING_AMOUNT = 32 * 1024
	// _MAX_PREALLOC is the biggest track the buffer is sized for in advance.
	// The size of a track is known from the response, so the buffer can be
	// allocated once instead of growing by copies, which keeps the peak
	// memory of a download close to the size of the track itself.
	_MAX_PREALLOC = 32 << 20
)

var errOutOfSize = errors.New("position is out of data size")

// BufferedStream downloads a track in the background and serves it to the
// player from memory. The network is only ever touched by the prefetch
// goroutine and never while the lock is held: a slow read must not block the
// UI, which asks for the progress and closes the stream when the track is
// switched.
type BufferedStream struct {
	source    io.ReadCloser
	totalSize int64

	mux        sync.Mutex
	cond       *sync.Cond
	readBuffer []byte
	readIndex  int64
	buffered   bool
	done       bool
	closed     bool
	lastError  error

	stop      chan struct{}
	stopOnce  sync.Once
	closeOnce sync.Once
	closeErr  error
}

func NewBufferedStream(source io.ReadCloser, totalSize int64) *BufferedStream {
	rs := &BufferedStream{
		source:    source,
		totalSize: totalSize,
		stop:      make(chan struct{}),
	}
	rs.cond = sync.NewCond(&rs.mux)

	go rs.prefetch()
	return rs
}

func (h *BufferedStream) Length() int64 {
	return h.totalSize
}

// Close stops the download and releases the stream. The source is closed
// before the lock is taken: the prefetch goroutine may be blocked in a slow
// network read, and waiting for it here is what froze the UI for seconds when
// the track was switched.
func (h *BufferedStream) Close() error {
	err := h.closeSource()

	h.mux.Lock()
	h.closed = true
	released := len(h.readBuffer)
	h.readBuffer = nil
	h.cond.Broadcast()
	h.mux.Unlock()

	releaseMemory(released)

	return err
}

func (h *BufferedStream) closeSource() error {
	h.closeOnce.Do(func() {
		h.stopOnce.Do(func() {
			close(h.stop)
		})
		if h.source != nil {
			h.closeErr = h.source.Close()
		}
	})
	return h.closeErr
}

func (h *BufferedStream) Read(dest []byte) (int, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	for {
		if h.closed {
			return 0, io.EOF
		}

		if h.readIndex < int64(len(h.readBuffer)) {
			n := copy(dest, h.readBuffer[h.readIndex:])
			h.readIndex += int64(n)
			if h.totalSize > 0 && h.readIndex >= h.totalSize {
				h.done = true
			}
			return n, nil
		}

		if h.buffered {
			// everything that was downloaded has been played
			h.done = true
			if h.lastError != nil && h.lastError != io.EOF {
				return 0, h.lastError
			}
			return 0, io.EOF
		}

		// nothing is buffered at the current position yet: wait for the
		// prefetch goroutine to bring more data
		h.cond.Wait()
	}
}

func (h *BufferedStream) Seek(offset int64, whence int) (pos int64, err error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = h.readIndex + offset
	case io.SeekEnd:
		pos = h.totalSize + offset
	}

	if pos < 0 || pos > h.totalSize {
		return h.readIndex, errOutOfSize
	}

	h.readIndex = pos
	h.done = pos >= h.totalSize
	h.cond.Broadcast()
	return pos, nil
}

func (h *BufferedStream) IsDone() bool {
	if h == nil {
		return false
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.done
}

func (h *BufferedStream) IsBuffered() bool {
	if h == nil {
		return false
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.buffered
}

func (h *BufferedStream) Progress() float64 {
	if h == nil || h.totalSize <= 0 {
		return 0
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return float64(h.readIndex) / float64(h.totalSize)
}

func (h *BufferedStream) BufferingProgress() float64 {
	if h == nil || h.totalSize <= 0 {
		return 0
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	return float64(len(h.readBuffer)) / float64(h.totalSize)
}

// BufferAll waits until the whole track is downloaded. It is used by the
// explicit "cache this track" action: the caller writes the buffer to disk
// right after.
func (h *BufferedStream) BufferAll() {
	h.mux.Lock()
	defer h.mux.Unlock()
	for !h.buffered && !h.closed {
		h.cond.Wait()
	}
}

func (h *BufferedStream) WriteTo(dest io.Writer) (int64, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	n, err := dest.Write(h.readBuffer)
	return int64(n), err
}

func (h *BufferedStream) Error() error {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.lastError
}

// isStopped reports whether Close has asked the prefetch goroutine to stop.
func (h *BufferedStream) isStopped() bool {
	select {
	case <-h.stop:
		return true
	default:
		return false
	}
}

// prefetch downloads the track as fast as the network allows. It never keeps
// the lock while reading, so the player, the progress bar and Close are not
// waiting for the network.
func (h *BufferedStream) prefetch() {
	buf := make([]byte, _BUFFERING_AMOUNT)

	if h.totalSize > 0 && h.totalSize <= _MAX_PREALLOC {
		h.mux.Lock()
		if !h.closed {
			h.readBuffer = make([]byte, 0, h.totalSize)
		}
		h.mux.Unlock()
	}

	for {
		select {
		case <-h.stop:
			return
		default:
		}

		n, err := h.source.Read(buf)

		h.mux.Lock()
		if n > 0 {
			h.readBuffer = append(h.readBuffer, buf[:n]...)
		}
		stopped := h.closed || h.isStopped()
		finished := err != nil || stopped || (h.totalSize > 0 && int64(len(h.readBuffer)) >= h.totalSize)
		if finished {
			h.buffered = true
			if !stopped && err != nil && err != io.EOF {
				h.lastError = err
			}
		}
		h.cond.Broadcast()
		h.mux.Unlock()

		if finished {
			return
		}

		if n == 0 {
			// a reader that returns no data and no error must not spin
			time.Sleep(time.Millisecond)
		}
	}
}
