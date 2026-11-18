package tracker

import (
	"errors"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pcmPrefetchChunkSize = 16 * 1024
	// About 3 seconds of PCM at 44.1 kHz, stereo S16.
	// This bounded queue uses about 512 KiB, not a track-sized allocation.
	pcmPrefetchChunks = 32
	// Do not unmute playback until the decoder has a useful head start.
	pcmStartupChunks = 8
)

type pcmSource interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Length() int64
	Progress() float64
	SourceProgress() float64
}

type pcmChunk struct {
	data       []byte
	generation uint64
	start      float64
	end        float64
}

type pcmSeekRequest struct {
	position   int64
	generation uint64
}

// asyncPCMReader moves MP3 decoding, loudness metering and EQ away from Oto's
// real-time callback. Oto only drains a bounded queue and never waits for the
// decoder, network, seek operation or UI message loop.
type asyncPCMReader struct {
	source pcmSource

	blocks      chan pcmChunk
	stop        chan struct{}
	done        chan struct{}
	seekWake    chan struct{}
	seekMu      sync.Mutex
	pendingSeek *pcmSeekRequest

	generation  atomic.Uint64
	finishedGen atomic.Uint64
	endedGen    atomic.Uint64
	ready       atomic.Bool
	readyGen    atomic.Uint64
	closed      atomic.Bool
	progress    atomic.Uint64

	// current and currentOffset are owned by Oto's single reader goroutine.
	current       pcmChunk
	currentOffset int
	currentGen    uint64
	lastProgress  time.Time
}

func newAsyncPCMReader(source pcmSource) *asyncPCMReader {
	r := &asyncPCMReader{
		source:   source,
		blocks:   make(chan pcmChunk, pcmPrefetchChunks),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		seekWake: make(chan struct{}, 1),
	}
	r.generation.Store(1)
	r.progress.Store(math.Float64bits(0))
	go r.prefetch()
	return r
}

func (r *asyncPCMReader) prefetch() {
	defer close(r.done)
	leaveAudioScheduling := enterAudioScheduling()
	defer leaveAudioScheduling()

	buf := make([]byte, pcmPrefetchChunkSize)

	for {
		if r.isStopped() {
			return
		}

		if request := r.takePendingSeek(); request != nil {
			_, err := r.source.Seek(request.position, io.SeekStart)
			if request.generation == r.generation.Load() {
				if err != nil {
					if sender, ok := r.source.(interface{ sendSeekError(error) }); ok {
						sender.sendSeekError(err)
					}
					// The decoder rejected the seek. Re-prime from its current
					// position instead of leaving the player permanently muted.
				}
				if progress := sourceProgressFor(r.source); progress >= 0 {
					r.progress.Store(math.Float64bits(progress))
				}
			}
			continue
		}

		gen := r.generation.Load()
		start := r.source.SourceProgress()
		n, err := r.source.Read(buf)
		end := r.source.SourceProgress()

		// A read that was already in flight when the user scrubbed is stale.
		// Never publish it into the new playback generation.
		if gen != r.generation.Load() {
			continue
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			chunk := pcmChunk{data: data, generation: gen, start: start, end: end}
			select {
			case r.blocks <- chunk:
			case <-r.seekWake:
				continue
			case <-r.stop:
				return
			}
		}

		if err != nil {
			// Store the generation, not a bool. A late EOF from an old read
			// can never mark a newer seek as finished.
			r.finishedGen.Store(gen)
			if err == io.EOF {
				// Keep the worker available for a seek at or after the end.
				select {
				case <-r.seekWake:
					continue
				case <-r.stop:
					return
				}
			}
			return
		}

		if n == 0 {
			select {
			case <-r.seekWake:
				continue
			case <-r.stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
}

func sourceProgressFor(source pcmSource) float64 {
	return source.SourceProgress()
}

func (r *asyncPCMReader) takePendingSeek() *pcmSeekRequest {
	r.seekMu.Lock()
	defer r.seekMu.Unlock()
	request := r.pendingSeek
	r.pendingSeek = nil
	return request
}

func (r *asyncPCMReader) isStopped() bool {
	select {
	case <-r.stop:
		return true
	default:
		return false
	}
}

// Read never waits for decoding or the network. During startup and after a
// seek it outputs silence until the queue is primed; a brief later underrun
// also produces silence rather than blocking Oto's real-time callback.
func (r *asyncPCMReader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if r.closed.Load() {
		return 0, io.EOF
	}

	gen := r.generation.Load()
	if r.currentGen != gen {
		r.current = pcmChunk{}
		r.currentOffset = 0
		r.currentGen = gen
	}

	if r.currentOffset >= len(r.current.data) && r.finishedGen.Load() == gen && len(r.blocks) == 0 {
		r.notifyEnded(gen)
		return 0, io.EOF
	}

	if !r.ready.Load() || r.readyGen.Load() != gen {
		if len(r.blocks) < pcmStartupChunks && r.finishedGen.Load() != gen {
			clear(dst)
			r.notifyProgress()
			return len(dst), nil
		}
		r.signalReady(gen)
	}

	written := 0
	for written < len(dst) {
		// A seek may arrive while Oto is consuming this buffer. Never emit
		// stale PCM or publish its progress after a newer generation exists.
		if r.generation.Load() != gen {
			clear(dst[written:])
			r.notifyProgress()
			return len(dst), nil
		}
		if r.currentOffset < len(r.current.data) && r.current.generation == gen {
			remaining := r.current.data[r.currentOffset:]
			n := copy(dst[written:], remaining)
			r.currentOffset += n
			written += n

			fraction := float64(r.currentOffset) / float64(len(r.current.data))
			progress := r.current.start + (r.current.end-r.current.start)*fraction
			r.progress.Store(math.Float64bits(progress))
			continue
		}

		r.current = pcmChunk{}
		r.currentOffset = 0

		select {
		case chunk := <-r.blocks:
			if chunk.generation != gen {
				continue
			}
			r.current = chunk
		default:
			if r.finishedGen.Load() == gen && len(r.blocks) == 0 {
				if written > 0 {
					return written, nil
				}
				r.notifyEnded(gen)
				return 0, io.EOF
			}

			clear(dst[written:])
			r.notifyProgress()
			return len(dst), nil
		}
	}

	r.notifyProgress()
	return written, nil
}

func (r *asyncPCMReader) signalReady(gen uint64) {
	if gen != r.generation.Load() || r.readyGen.Load() == gen {
		return
	}
	r.ready.Store(true)
	r.readyGen.Store(gen)
	if sender, ok := r.source.(interface{ sendReady() }); ok {
		sender.sendReady()
	}
}

func (r *asyncPCMReader) IsReady() bool {
	return r.ready.Load()
}

func (r *asyncPCMReader) notifyProgress() {
	if r.source == nil || time.Since(r.lastProgress) < _PROGRESS_UPDATE_PERIOD {
		return
	}
	r.lastProgress = time.Now()
	if sender, ok := r.source.(interface{ sendProgress(float64) }); ok {
		sender.sendProgress(math.Float64frombits(r.progress.Load()))
	}
}

func (r *asyncPCMReader) notifyEnded(gen uint64) {
	if gen != r.generation.Load() || r.endedGen.Load() == gen {
		return
	}
	previous := r.endedGen.Load()
	if !r.endedGen.CompareAndSwap(previous, gen) || gen != r.generation.Load() {
		return
	}
	if sender, ok := r.source.(interface{ sendEnded() }); ok {
		sender.sendEnded()
	}
}

// Seek is deliberately non-blocking. It invalidates queued PCM immediately
// and asks the decoder worker to perform the actual seek once its current
// network-backed read has returned. Repeated scrubs collapse to the newest one.
func (r *asyncPCMReader) Seek(offset int64, whence int) (int64, error) {
	if r.closed.Load() {
		return 0, io.EOF
	}
	length := r.source.Length()
	if length <= 0 {
		return 0, errors.New("track length is unavailable")
	}

	current := int64(math.Float64frombits(r.progress.Load()) * float64(length))
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = current + offset
	case io.SeekEnd:
		target = length + offset
	default:
		return current, errors.New("invalid seek origin")
	}
	if target < 0 || target > length {
		return current, errors.New("seek position is outside the track")
	}
	target -= target % 4

	gen := r.generation.Add(1)
	r.ready.Store(false)
	r.finishedGen.Store(0)
	r.progress.Store(math.Float64bits(float64(target) / float64(length)))

	// Drop old PCM immediately so it cannot play after the seek.
	for {
		select {
		case <-r.blocks:
		default:
			goto drained
		}
	}
drained:
	r.seekMu.Lock()
	r.pendingSeek = &pcmSeekRequest{position: target, generation: gen}
	r.seekMu.Unlock()

	select {
	case r.seekWake <- struct{}{}:
	default:
	}
	if sender, ok := r.source.(interface{ setProgress(float64) }); ok {
		sender.setProgress(float64(target) / float64(length))
	}
	return target, nil
}

func (r *asyncPCMReader) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(r.stop)
	<-r.done
	return nil
}

func (r *asyncPCMReader) Progress() float64 {
	return math.Float64frombits(r.progress.Load())
}
