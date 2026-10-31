package stream

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

// blockingReader blocks in Read until it is closed.
type blockingReader struct {
	closed chan struct{}
	once   sync.Once
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.closed
	return 0, io.ErrClosedPipe
}

func (r *blockingReader) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func readAll(t *testing.T, s *BufferedStream) []byte {
	t.Helper()

	var got []byte
	buf := make([]byte, 1000)
	for i := 0; i < 10000; i++ {
		n, err := s.Read(buf)
		got = append(got, buf[:n]...)
		if err == io.EOF {
			return got
		}
		if err != nil {
			t.Fatalf("read: %s", err)
		}
	}
	t.Fatal("read did not reach the end")
	return nil
}

func TestReadServesTheWholeTrack(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefgh"), 4096)
	s := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))

	if got := readAll(t, s); !bytes.Equal(got, data) {
		t.Fatalf("read %d bytes, want %d", len(got), len(data))
	}
	if !s.IsBuffered() {
		t.Error("the stream must report that it is buffered")
	}
	if !s.IsDone() {
		t.Error("the stream must report that it is done")
	}
	if progress := s.Progress(); progress != 1 {
		t.Errorf("progress = %f, want 1", progress)
	}
}

func TestSeekBackwardsUsesTheBufferedData(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789"), 1000)
	s := NewBufferedStream(io.NopCloser(bytes.NewReader(data)), int64(len(data)))

	head := make([]byte, 100)
	if _, err := io.ReadFull(s, head); err != nil {
		t.Fatalf("read head: %s", err)
	}
	if _, err := s.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek: %s", err)
	}

	again := make([]byte, 100)
	if _, err := io.ReadFull(s, again); err != nil {
		t.Fatalf("read after seek: %s", err)
	}
	if !bytes.Equal(again, head) {
		t.Error("the data after the seek differs from the first read")
	}
}

func TestSeekPastTheEndFails(t *testing.T) {
	s := NewBufferedStream(io.NopCloser(bytes.NewReader(make([]byte, 100))), 100)
	if _, err := s.Seek(200, io.SeekStart); err != errOutOfSize {
		t.Fatalf("seek error = %v, want %v", err, errOutOfSize)
	}
}

func TestCloseDoesNotWaitForTheNetwork(t *testing.T) {
	src := &blockingReader{closed: make(chan struct{})}
	s := NewBufferedStream(src, 1000)

	// let the prefetch goroutine block in Read
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close waited for the blocked prefetch")
	}
}

func TestCloseUnblocksARead(t *testing.T) {
	src := &blockingReader{closed: make(chan struct{})}
	s := NewBufferedStream(src, 1000)

	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 100)
		_, err := s.Read(buf)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	s.Close()

	select {
	case err := <-done:
		if err != io.EOF {
			t.Fatalf("read error = %v, want EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock the pending read")
	}
}

func TestBufferAllWaitsForTheTrack(t *testing.T) {
	data := bytes.Repeat([]byte("xyz"), 10000)
	s := NewBufferedStream(io.NopCloser(io.LimitReader(bytes.NewReader(data), int64(len(data)))), int64(len(data)))

	s.BufferAll()

	written, err := s.WriteTo(io.Discard)
	if err != nil {
		t.Fatalf("write: %s", err)
	}
	if written != int64(len(data)) {
		t.Fatalf("buffered %d bytes, want %d", written, len(data))
	}
}
