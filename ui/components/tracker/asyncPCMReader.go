package tracker

import (
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pcmPrefetchChunkSize = 16 * 1024
	pcmPrefetchChunks    = 32
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

// asyncPCMReader moves MP3 decoding, loudness metering and EQ away from Oto's
// real-time read callback. The bounded queue reserves about 3 seconds of
// stereo 44.1 kHz/16-bit audio without allowing memory to grow with track size.
type asyncPCMReader struct {
	source     pcmSource
	blocks     chan pcmChunk
	stop       chan struct{}
	done       chan struct{}
	seekSignal chan struct{}

	sourceMu sync.Mutex

	generation atomic.Uint64
	finished   atomic.Bool
	closed     atomic.Bool
	endSent    atomic.Bool
	progress   atomic.Uint64

	// current and currentOffset are owned by Oto's single reader goroutine.
	current       pcmChunk
	currentOffset int
	currentGen    uint64
	lastProgress  time.Time
}

func newAsyncPCMReader(source pcmSource) *asyncPCMReader {
	r := &asyncPCMReader{
		source:     source,
		blocks:     make(chan pcmChunk, pcmPrefetchChunks),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		seekSignal: make(chan struct{}, 1),
	}
	r.generation.Store(1)
	r.progress.Store(math.Float64bits(0))
	go r.prefetch()
	return r
}

func (r *asyncPCMReader) prefetch() {
	defer close(r.done)
	buf := make([]byte, pcmPrefetchChunkSize)

	for {
		select {
		case <-r.stop:
			return
		default:
		}

		r.sourceMu.Lock()
		gen := r.generation.Load()
		start := r.source.SourceProgress()
		n, err := r.source.Read(buf)
		end := r.source.SourceProgress()
		r.sourceMu.Unlock()

		if n > 0 && gen == r.generation.Load() {
			data := make([]byte, n)
			copy(data, buf[:n])
			chunk := pcmChunk{data: data, generation: gen, start: start, end: end}
			select {
			case r.blocks <- chunk:
			case <-r.stop:
				return
			}
		}

		// Seek may have started a new generation while the old read was
		// publishing its final result. Never let an old EOF finish the new one.
		if err != nil {
			if gen == r.generation.Load() {
				r.finished.Store(true)
			}
			if err == io.EOF {
				// Keep the worker asleep but available for a seek near the end:
				// Oto can still be draining the final queued PCM samples.
				select {
				case <-r.stop:
					return
				case <-r.seekSignal:
					continue
				}
			}
			return
		}
		if n == 0 {
			time.Sleep(time.Millisecond)
		}
	}
}

// Read never waits for the decoder or the network. If the producer briefly
// falls behind, the missing tail is silence rather than a blocked audio read.
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

	written := 0
	for written < len(dst) {
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
			if r.finished.Load() && len(r.blocks) == 0 {
				if written > 0 {
					return written, nil
				}
				r.notifyEnded()
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

func (r *asyncPCMReader) notifyProgress() {
	if r.source == nil || time.Since(r.lastProgress) < _PROGRESS_UPDATE_PERIOD {
		return
	}
	r.lastProgress = time.Now()
	if sender, ok := r.source.(interface{ sendProgress(float64) }); ok {
		sender.sendProgress(math.Float64frombits(r.progress.Load()))
	}
}

func (r *asyncPCMReader) notifyEnded() {
	if !r.endSent.CompareAndSwap(false, true) {
		return
	}
	if sender, ok := r.source.(interface{ sendEnded() }); ok {
		sender.sendEnded()
	}
}

func (r *asyncPCMReader) Seek(offset int64, whence int) (int64, error) {
	if r.closed.Load() {
		return 0, io.EOF
	}

	r.sourceMu.Lock()
	defer r.sourceMu.Unlock()

	pos, err := r.source.Seek(offset, whence)
	if err != nil {
		return pos, err
	}

	r.generation.Add(1)
	r.finished.Store(false)
	r.endSent.Store(false)
	for {
		select {
		case <-r.blocks:
		default:
			goto drained
		}
	}
drained:
	if length := r.source.Length(); length > 0 {
		r.progress.Store(math.Float64bits(float64(pos) / float64(length)))
	} else {
		r.progress.Store(math.Float64bits(0))
	}
	select {
	case r.seekSignal <- struct{}{}:
	default:
	}
	return pos, nil
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
