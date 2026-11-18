package mp3

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/dece2183/go-stream-mp3/internal/testaudio"
)

func TestSeekIndexesOnlyAsFarAsTarget(t *testing.T) {
	data := testaudio.Build(testaudio.Options{Frames: 400, XingMode: testaudio.Xing})
	d, err := NewDecoderWithOptions(bytes.NewReader(data), DecoderOptions{})
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	if len(d.frameStarts) != 0 || d.scanned {
		t.Fatalf("opening should not build the frame index: frames=%d scanned=%v", len(d.frameStarts), d.scanned)
	}

	targetSample := d.SampleCount() / 2
	targetTime := time.Duration(targetSample) * time.Second / time.Duration(d.SampleRate())
	if err := d.SeekToSample(targetSample); err != nil {
		t.Fatalf("SeekToSample: %v", err)
	}
	if got := d.Position(); got < targetTime-10*time.Millisecond || got > targetTime+10*time.Millisecond {
		t.Fatalf("position after seek = %v, want approximately %v", got, targetTime)
	}
	if len(d.frameStarts) < 150 || len(d.frameStarts) > 220 {
		t.Fatalf("first seek indexed %d frames; want only the prefix near the midpoint", len(d.frameStarts))
	}
	if d.scanned {
		t.Fatal("midpoint seek scanned the entire file")
	}

	before := len(d.frameStarts)
	if err := d.SeekToSample(d.SampleCount() / 4); err != nil {
		t.Fatalf("second seek: %v", err)
	}
	if len(d.frameStarts) != before {
		t.Fatalf("backward seek rebuilt the index: frames %d -> %d", before, len(d.frameStarts))
	}
	buf := make([]byte, 4096)
	if n, err := d.Read(buf); err != nil || n == 0 {
		t.Fatalf("read after seek: n=%d err=%v", n, err)
	}
}

func TestSeekIndexesOnlyToTargetWithoutXingAndHandlesEnd(t *testing.T) {
	data := testaudio.Build(testaudio.Options{
		Frames:       400,
		BitratesKbps: []int{64, 128, 192},
	})
	d, err := NewDecoderWithOptions(bytes.NewReader(data), DecoderOptions{})
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	if d.length != invalidLength {
		t.Fatalf("length without Xing header should be unknown, got %d", d.length)
	}
	target := d.bytesPerFrame * 120
	if pos, err := d.Seek(target, io.SeekStart); err != nil || pos != target {
		t.Fatalf("VBR seek: pos=%d err=%v, want %d", pos, err, target)
	}
	if len(d.frameStarts) < 120 || len(d.frameStarts) > 125 || d.scanned {
		t.Fatalf("VBR partial index: frames=%d scanned=%v", len(d.frameStarts), d.scanned)
	}
	buf := make([]byte, 4096)
	if n, err := d.Read(buf); err != nil || n == 0 {
		t.Fatalf("VBR read after seek: n=%d err=%v", n, err)
	}

	d2, err := NewDecoderWithOptions(bytes.NewReader(data), DecoderOptions{})
	if err != nil {
		t.Fatalf("second NewDecoder: %v", err)
	}
	pastEnd := d2.bytesPerFrame * 500
	pos, err := d2.Seek(pastEnd, io.SeekStart)
	if err != nil {
		t.Fatalf("seek past end: %v", err)
	}
	wantEnd := int64(400) * d2.bytesPerFrame
	if pos != wantEnd || d2.Length() != wantEnd {
		t.Fatalf("seek past end = %d, length = %d, want %d", pos, d2.Length(), wantEnd)
	}
	if !d2.scanned {
		t.Fatal("seeking past EOF should have discovered the actual stream end")
	}
}

func TestSeekToKnownEndDoesNotBuildIndex(t *testing.T) {
	data := testaudio.Build(testaudio.Options{Frames: 400, XingMode: testaudio.Xing})
	d, err := NewDecoderWithOptions(bytes.NewReader(data), DecoderOptions{})
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	end := d.Length()
	if _, err := d.Seek(end, io.SeekStart); err != nil {
		t.Fatalf("seek to end: %v", err)
	}
	if len(d.frameStarts) != 0 || d.scanned {
		t.Fatalf("seek to known end scanned frames: frames=%d scanned=%v", len(d.frameStarts), d.scanned)
	}
}
