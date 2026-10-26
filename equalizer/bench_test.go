package equalizer

import "testing"

// BenchmarkApplyEQ measures the cost of the DSP on one second of stereo
// 16-bit audio at 44.1 kHz with all ten bands active.
func BenchmarkApplyEQ(b *testing.B) {
	const rate = 44100
	pcm := make([]byte, rate*4) // one second of stereo s16le

	// a simple sine so the filters do real work
	for i := 0; i+3 < len(pcm); i += 4 {
		v := int16(12000)
		pcm[i] = byte(v)
		pcm[i+1] = byte(v >> 8)
		pcm[i+2] = byte(v)
		pcm[i+3] = byte(v >> 8)
	}

	p := New()
	p.SetRate(rate)
	var gains [Bands]float64
	for i := range gains {
		gains[i] = 3
	}
	p.Set(true, gains)

	b.SetBytes(int64(len(pcm)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Apply(pcm)
	}
}
