package tracker

import (
	"bytes"
	"io"
	"math"
	"sync"
	"testing"
	"time"
)

type delayedPCMSource struct {
	mu      sync.Mutex
	data    []byte
	offset  int
	started chan struct{}
	release chan struct{}
	once    sync.Once
	ended   chan struct{}
}

func newDelayedPCMSource(data []byte) *delayedPCMSource {
	return &delayedPCMSource{
		data:    data,
		started: make(chan struct{}),
		release: make(chan struct{}),
		ended:   make(chan struct{}, 1),
	}
}

func (s *delayedPCMSource) Read(dst []byte) (int, error) {
	s.once.Do(func() {
		close(s.started)
		<-s.release
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.offset >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(dst, s.data[s.offset:])
	s.offset += n
	return n, nil
}

func (s *delayedPCMSource) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch whence {
	case io.SeekStart:
		s.offset = int(offset)
	case io.SeekCurrent:
		s.offset += int(offset)
	case io.SeekEnd:
		s.offset = len(s.data) + int(offset)
	}
	if s.offset < 0 || s.offset > len(s.data) {
		return int64(s.offset), io.ErrUnexpectedEOF
	}
	return int64(s.offset), nil
}

func (s *delayedPCMSource) Length() int64 { return int64(len(s.data)) }

func (s *delayedPCMSource) SourceProgress() float64 { return s.Progress() }

func (s *delayedPCMSource) Progress() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data) == 0 {
		return 0
	}
	return float64(s.offset) / float64(len(s.data))
}

func (s *delayedPCMSource) sendProgress(float64) {}

func (s *delayedPCMSource) sendEnded() {
	select {
	case s.ended <- struct{}{}:
	default:
	}
}

func TestAsyncPCMReaderUsesAbsoluteSeekPosition(t *testing.T) {
	data := make([]byte, 128)
	for i := range data {
		data[i] = byte(i)
	}
	source := newDelayedPCMSource(data)
	reader := newAsyncPCMReader(source)
	defer func() {
		select {
		case <-source.release:
		default:
			close(source.release)
		}
		_ = reader.Close()
	}()

	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("decoder worker did not start")
	}

	const target = 64
	if pos, err := reader.Seek(target, io.SeekStart); err != nil || pos != target {
		t.Fatalf("absolute seek = %d, %v; want %d", pos, err, target)
	}
	if got := math.Float64frombits(reader.progress.Load()); got != 0.5 {
		t.Fatalf("progress after seek = %v, want 0.5", got)
	}

	close(source.release)
	want := data[target:]
	dst := make([]byte, len(want))
	deadline := time.After(time.Second)
	for {
		n, err := reader.Read(dst)
		if n == len(want) && err == nil && bytes.Equal(dst, want) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("PCM after absolute seek = %v, n=%d err=%v; want %v", dst, n, err, want)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestAsyncPCMReaderCanSeekAfterEOF(t *testing.T) {
	source := newDelayedPCMSource([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	close(source.release)
	reader := newAsyncPCMReader(source)
	defer reader.Close()

	dst := make([]byte, 8)
	deadline := time.After(time.Second)
	for {
		n, err := reader.Read(dst)
		if n == len(dst) && err == nil && bytes.Equal(dst, source.data) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("first PCM read timed out: %d, %v", n, err)
		case <-time.After(time.Millisecond):
		}
	}

	if err := waitForPCMEOF(reader, dst); err != nil {
		t.Fatalf("expected EOF before seek: %v", err)
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek after EOF: %v", err)
	}

	deadline = time.After(time.Second)
	for {
		n, err := reader.Read(dst)
		if n == len(dst) && err == nil && bytes.Equal(dst, source.data) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("PCM did not resume after seek: %d, %v", n, err)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestAsyncPCMReaderDoesNotWaitForDecoder(t *testing.T) {
	source := newDelayedPCMSource([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	reader := newAsyncPCMReader(source)
	defer func() {
		select {
		case <-source.release:
		default:
			close(source.release)
		}
		_ = reader.Close()
	}()

	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("decoder worker did not start")
	}

	dst := make([]byte, 8)
	start := time.Now()
	n, err := reader.Read(dst)
	if err != nil || n != len(dst) {
		t.Fatalf("non-blocking read = %d, %v", n, err)
	}
	if time.Since(start) > 25*time.Millisecond {
		t.Fatalf("audio read waited for the decoder: %s", time.Since(start))
	}
	if !bytes.Equal(dst, make([]byte, len(dst))) {
		t.Fatalf("empty queue should return silence, got %v", dst)
	}

	close(source.release)
	deadline := time.After(time.Second)
	for {
		n, err = reader.Read(dst)
		if n == len(dst) && err == nil && bytes.Equal(dst, source.data) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("decoded PCM never reached the reader: %v, %v", dst, err)
		case <-time.After(time.Millisecond):
		}
	}

	if err := waitForPCMEOF(reader, dst); err != nil {
		t.Fatalf("read after queued audio should reach EOF: %v", err)
	}
	select {
	case <-source.ended:
	case <-time.After(time.Second):
		t.Fatal("end notification was not sent after the queued audio drained")
	}
}

func waitForPCMEOF(reader *asyncPCMReader, dst []byte) error {
	deadline := time.After(time.Second)
	for {
		n, err := reader.Read(dst)
		if n == 0 && err == io.EOF {
			return nil
		}
		select {
		case <-deadline:
			return io.ErrNoProgress
		case <-time.After(time.Millisecond):
		}
	}
}
