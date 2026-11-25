// Package visualizer renders the audio reactive background of "My Wave".
//
// The animation is a Go port of the WGSL compute shader of yamusic
// (src/app/components/visualizer_gpu.rs, commit 74186269c50e6aa1dc3e1c23b4df138057db4580).
// The shader renders three layered noise rings with sparkles; the terminal
// version keeps the same geometry and colours, and uses the upper half block
// "▀" with the foreground colour of the top half cell and the background
// colour of the bottom one, exactly like the original renderer.
//
// Rendering runs in a single worker goroutine. The UI asks for the latest
// ready frame and never blocks on it: the worker publishes one frame at a
// time (a bounded queue of one), so a slow terminal simply skips frames
// instead of accumulating work.
package visualizer

import (
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// RGB is one colour of the palette.
type RGB struct {
	R, G, B uint8
}

// Palette is the target palette of the visualizer: the backdrop and the six
// bands of the three rings. Every render interpolates towards it, so a track
// change fades instead of jumping.
type Palette struct {
	Bg     RGB
	Colors [6]RGB
}

// Frame is one rendered block of text.
type Frame struct {
	Text string
	// Generation grows with every produced frame, so the UI can tell a new
	// frame from the previous one without comparing the whole string.
	Generation uint64
}

const (
	// DefaultFramePeriod is the target frame time at full speed. It is close
	// to the 13 ms of the original renderer but leaves the CPU room for the
	// audio thread.
	DefaultFramePeriod = 50 * time.Millisecond

	// idleFramePeriod is used while the playback is paused: the rings keep
	// moving slowly, the CPU cost drops.
	idleFramePeriod = 160 * time.Millisecond
)

// Renderer owns the worker goroutine and the latest frame.
type Renderer struct {
	mu      sync.Mutex
	width   int
	height  int
	palette Palette
	glow    float64
	enabled bool

	amplitude uint32 // math.Float32bits
	playing   uint32

	frameMu sync.RWMutex
	frame   Frame

	lifeMu  sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	running bool

	// now is replaceable in tests.
	now func() time.Time
}

// New creates a stopped renderer. Call SetEnabled(true) to start it.
func New() *Renderer {
	return &Renderer{
		now: time.Now,
		palette: Palette{
			Bg: RGB{13, 13, 13},
			Colors: [6]RGB{
				{247, 212, 75}, {255, 160, 80}, {200, 160, 255},
				{90, 200, 255}, {255, 140, 200}, {120, 255, 200},
			},
		},
	}
}

// SetEnabled starts or stops the worker. Disabling releases the goroutine and
// the last frame, so the resource saving mode really frees the resources.
func (r *Renderer) SetEnabled(enabled bool) {
	r.lifeMu.Lock()
	defer r.lifeMu.Unlock()

	if r.enabled == enabled {
		return
	}
	r.enabled = enabled

	if !enabled {
		r.stopLocked()
		r.frameMu.Lock()
		r.frame = Frame{}
		r.frameMu.Unlock()
		return
	}
	r.startLocked()
}

// Enabled reports whether the worker is running.
func (r *Renderer) Enabled() bool {
	r.lifeMu.Lock()
	defer r.lifeMu.Unlock()
	return r.enabled
}

func (r *Renderer) startLocked() {
	if r.running {
		return
	}
	r.stop = make(chan struct{})
	r.done = make(chan struct{})
	r.running = true
	stop, done := r.stop, r.done
	go r.work(stop, done)
}

func (r *Renderer) stopLocked() {
	if !r.running {
		return
	}
	close(r.stop)
	done := r.done
	r.running = false
	r.stop, r.done = nil, nil
	// The worker is not holding any lock the caller needs, so waiting for it
	// is safe and makes "disabled" mean "nothing of it is left".
	<-done
}

// Stop releases the worker.
func (r *Renderer) Stop() { r.SetEnabled(false) }

// Resize tells the renderer the size of the block, in terminal cells.
func (r *Renderer) Resize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	// 240x40 is more than any sane player area; the cap keeps a corrupted
	// terminal size from allocating gigabytes.
	if width > 400 {
		width = 400
	}
	if height > 40 {
		height = 40
	}

	r.mu.Lock()
	if r.width != width || r.height != height {
		r.width, r.height = width, height
		// The old frame has a different width: the UI must not draw it.
		r.frameMu.Lock()
		r.frame = Frame{}
		r.frameMu.Unlock()
	}
	r.mu.Unlock()
}

// SetPalette sets the target palette. The worker fades to it.
func (r *Renderer) SetPalette(palette Palette) {
	r.mu.Lock()
	r.palette = palette
	r.mu.Unlock()
}

// SetAmplitude feeds the current audio level, 0..1.
func (r *Renderer) SetAmplitude(amplitude float64) {
	if amplitude < 0 {
		amplitude = 0
	}
	if amplitude > 1 {
		amplitude = 1
	}
	atomic.StoreUint32(&r.amplitude, math.Float32bits(float32(amplitude)))
}

// SetPlaying tells the renderer whether music is playing: paused playback
// uses a slower loop and leaves the rings moving gently.
func (r *Renderer) SetPlaying(playing bool) {
	if playing {
		atomic.StoreUint32(&r.playing, 1)
	} else {
		atomic.StoreUint32(&r.playing, 0)
	}
}

// TriggerGlow starts the short bright pulse used when the track is liked.
func (r *Renderer) TriggerGlow() {
	r.mu.Lock()
	r.glow = 1
	r.mu.Unlock()
}

// Latest returns the last finished frame. The second value is false while no
// frame is ready.
func (r *Renderer) Latest() (Frame, bool) {
	r.frameMu.RLock()
	frame := r.frame
	r.frameMu.RUnlock()
	return frame, frame.Text != ""
}

func (r *Renderer) work(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	state := animState{
		speed: 0.8,
		bg:    [3]float64{13.0 / 255, 13.0 / 255, 13.0 / 255},
	}
	// The first palette is applied at once: the panel must not open black and
	// only then catch up with the theme.
	r.mu.Lock()
	target := r.palette
	r.mu.Unlock()
	for i := range state.palette {
		state.palette[i] = [3]float64{
			float64(target.Colors[i].R) / 255,
			float64(target.Colors[i].G) / 255,
			float64(target.Colors[i].B) / 255,
		}
	}
	state.bg = [3]float64{
		float64(target.Bg.R) / 255,
		float64(target.Bg.G) / 255,
		float64(target.Bg.B) / 255,
	}

	last := r.now()
	ticker := time.NewTicker(DefaultFramePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		now := r.now()
		dt := now.Sub(last).Seconds()
		last = now
		if dt <= 0 {
			continue
		}
		if dt > 0.1 {
			dt = 0.1
		}

		r.mu.Lock()
		width, height := r.width, r.height
		target := r.palette
		glow := r.glow
		r.mu.Unlock()

		playing := atomic.LoadUint32(&r.playing) != 0
		raw := float64(math.Float32frombits(atomic.LoadUint32(&r.amplitude)))

		// The same smoothing as the original widget: a loud attack rises
		// slowly, a quiet tail falls even slower, so the rings breathe.
		if raw > state.amplitude {
			state.amplitude = state.amplitude*0.85 + raw*0.15
		} else {
			state.amplitude = state.amplitude*0.95 + raw*0.05
		}

		if glow > 0 {
			glowTarget := glow - dt*0.4
			if glowTarget < 0 {
				glowTarget = 0
			}
			r.mu.Lock()
			r.glow = glowTarget
			r.mu.Unlock()
		}
		glowLerp := dt * 2.0
		if glow > state.glow {
			glowLerp = dt * 5.0
		}
		state.glow += (glow - state.glow) * math.Min(1, glowLerp)

		speedTarget := 0.8
		if playing {
			speedTarget = 0.8 + state.amplitude*1.5
		}
		state.speed += (speedTarget - state.speed) * math.Min(1, dt*3)

		// palette transition, dt*6 like the original
		lerp := math.Min(1, dt*6)
		for i := range state.palette {
			c := target.Colors[i]
			state.palette[i][0] += (float64(c.R)/255 - state.palette[i][0]) * lerp
			state.palette[i][1] += (float64(c.G)/255 - state.palette[i][1]) * lerp
			state.palette[i][2] += (float64(c.B)/255 - state.palette[i][2]) * lerp
		}
		state.bg[0] += (float64(target.Bg.R)/255 - state.bg[0]) * lerp
		state.bg[1] += (float64(target.Bg.G)/255 - state.bg[1]) * lerp
		state.bg[2] += (float64(target.Bg.B)/255 - state.bg[2]) * lerp

		state.phase += dt * state.speed
		if width <= 0 || height <= 0 {
			continue
		}

		text := renderFrame(width, height, state)

		r.frameMu.Lock()
		r.frame.Generation++
		r.frame.Text = text
		r.frameMu.Unlock()

		// paused playback does not need 20 frames per second
		if !playing {
			select {
			case <-stop:
				return
			case <-time.After(idleFramePeriod - DefaultFramePeriod):
			}
		}
	}
}

type animState struct {
	phase     float64
	amplitude float64
	speed     float64
	glow      float64
	palette   [6][3]float64
	bg        [3]float64
}

type vec2 struct{ X, Y float64 }
type vec3 struct{ X, Y, Z float64 }
type vec4 struct{ X, Y, Z, W float64 }

func (v vec2) add(o vec2) vec2    { return vec2{v.X + o.X, v.Y + o.Y} }
func (v vec2) mul(s float64) vec2 { return vec2{v.X * s, v.Y * s} }
func (v vec3) add(o vec3) vec3    { return vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v vec3) mul(s float64) vec3 { return vec3{v.X * s, v.Y * s, v.Z * s} }
func (v vec3) dot(o vec3) float64 { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }
func (v vec4) dot(o vec4) float64 { return v.X*o.X + v.Y*o.Y + v.Z*o.Z + v.W*o.W }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func fract(v float64) float64 { return v - math.Floor(v) }

func smoothstep(edge0, edge1, x float64) float64 {
	t := clamp((x-edge0)/(edge1-edge0), 0, 1)
	return t * t * (3 - 2*t)
}

func mix(a, b, t float64) float64 { return a + (b-a)*t }

func mod289(v float64) float64 { return v - math.Floor(v/289)*289 }

func perm4(v vec4) vec4 {
	return vec4{
		mod289(((v.X * 34) + 1) * v.X),
		mod289(((v.Y * 34) + 1) * v.Y),
		mod289(((v.Z * 34) + 1) * v.Z),
		mod289(((v.W * 34) + 1) * v.W),
	}
}

func stepv(edge, x vec4) vec4 {
	s := func(e, v float64) float64 {
		if v < e {
			return 0
		}
		return 1
	}
	return vec4{s(edge.X, x.X), s(edge.Y, x.Y), s(edge.Z, x.Z), s(edge.W, x.W)}
}

func minv(a, b vec4) vec4 {
	return vec4{math.Min(a.X, b.X), math.Min(a.Y, b.Y), math.Min(a.Z, b.Z), math.Min(a.W, b.W)}
}

func maxv(a, b vec4) vec4 {
	return vec4{math.Max(a.X, b.X), math.Max(a.Y, b.Y), math.Max(a.Z, b.Z), math.Max(a.W, b.W)}
}

func absv(v vec4) vec4 {
	return vec4{math.Abs(v.X), math.Abs(v.Y), math.Abs(v.Z), math.Abs(v.W)}
}

func floorv(v vec4) vec4 {
	return vec4{math.Floor(v.X), math.Floor(v.Y), math.Floor(v.Z), math.Floor(v.W)}
}

// simplex3 is the simplex noise of the shader.
func simplex3(p vec3) float64 {
	const c0 = 0.1666667
	const c1 = 0.3333333

	dotC := (p.X + p.Y + p.Z) * c1
	i := vec3{math.Floor(p.X + dotC), math.Floor(p.Y + dotC), math.Floor(p.Z + dotC)}
	dotI := (i.X + i.Y + i.Z) * c0
	x0 := vec3{p.X - i.X + dotI, p.Y - i.Y + dotI, p.Z - i.Z + dotI}

	g := stepv(vec4{x0.Y, x0.Z, x0.X, 0}, vec4{x0.X, x0.Y, x0.Z, 0})
	l := vec4{1 - g.X, 1 - g.Y, 1 - g.Z, 1 - g.W}
	i1 := minv(vec4{g.X, g.Y, g.Z, 0}, vec4{l.Z, l.X, l.Y, 0})
	i2 := maxv(vec4{g.X, g.Y, g.Z, 0}, vec4{l.Z, l.X, l.Y, 0})

	x1 := vec3{x0.X - i1.X + c0, x0.Y - i1.Y + c0, x0.Z - i1.Z + c0}
	x2 := vec3{x0.X - i2.X + c1, x0.Y - i2.Y + c1, x0.Z - i2.Z + c1}
	x3 := vec3{x0.X - 0.5, x0.Y - 0.5, x0.Z - 0.5}

	i.X = mod289(i.X)
	i.Y = mod289(i.Y)
	i.Z = mod289(i.Z)

	// p_hash = perm(perm(perm(i.z + vec4(0, i1.z, i2.z, 1))
	//                     + i.y + vec4(0, i1.y, i2.y, 1))
	//                     + i.x + vec4(0, i1.x, i2.x, 1))
	pHash := perm4(vec4{
		i.Z + 0,
		i.Z + i1.Z,
		i.Z + i2.Z,
		i.Z + 1,
	})
	pHash = perm4(pHash.addVec4(vec4{i.Y, i.Y, i.Y, i.Y}).addVec4(vec4{0, i1.Y, i2.Y, 1}))
	pHash = perm4(pHash.addVec4(vec4{i.X, i.X, i.X, i.X}).addVec4(vec4{0, i1.X, i2.X, 1}))

	const n = 0.142857142857
	// ns = n_ * D.wyz - D.xzx, with D = (0, .5, 1, 2)
	nsX := n * 2
	nsY := n*0.5 - 1
	nsZ := n

	j := vec4{
		pHash.X - 49*math.Floor(pHash.X*nsZ*nsZ),
		pHash.Y - 49*math.Floor(pHash.Y*nsZ*nsZ),
		pHash.Z - 49*math.Floor(pHash.Z*nsZ*nsZ),
		pHash.W - 49*math.Floor(pHash.W*nsZ*nsZ),
	}
	xs := floorv(vec4{j.X * nsZ, j.Y * nsZ, j.Z * nsZ, j.W * nsZ})
	ys := floorv(vec4{j.X - 7*xs.X, j.Y - 7*xs.Y, j.Z - 7*xs.Z, j.W - 7*xs.W})

	x := vec4{xs.X*nsX + nsY, xs.Y*nsX + nsY, xs.Z*nsX + nsY, xs.W*nsX + nsY}
	y := vec4{ys.X*nsX + nsY, ys.Y*nsX + nsY, ys.Z*nsX + nsY, ys.W*nsX + nsY}
	h := vec4{1 - math.Abs(x.X) - math.Abs(y.X), 1 - math.Abs(x.Y) - math.Abs(y.Y),
		1 - math.Abs(x.Z) - math.Abs(y.Z), 1 - math.Abs(x.W) - math.Abs(y.W)}

	b0 := vec4{x.X, x.Y, y.X, y.Y}
	b1 := vec4{x.Z, x.W, y.Z, y.W}

	s0 := vec4{math.Floor(b0.X)*2 + 1, math.Floor(b0.Y)*2 + 1, math.Floor(b0.Z)*2 + 1, math.Floor(b0.W)*2 + 1}
	s1 := vec4{math.Floor(b1.X)*2 + 1, math.Floor(b1.Y)*2 + 1, math.Floor(b1.Z)*2 + 1, math.Floor(b1.W)*2 + 1}

	sh := func(v float64) float64 {
		if v <= 0 {
			return -1
		}
		return 0
	}
	shv := vec4{sh(h.X), sh(h.Y), sh(h.Z), sh(h.W)}

	a0 := vec4{
		b0.X + s0.X*shv.X,
		b0.Z + s0.Z*shv.X,
		b0.Y + s0.Y*shv.Y,
		b0.W + s0.W*shv.Y,
	}
	a1 := vec4{
		b1.X + s1.X*shv.Z,
		b1.Z + s1.Z*shv.Z,
		b1.Y + s1.Y*shv.W,
		b1.W + s1.W*shv.W,
	}

	q0 := vec3{a0.X, a0.Y, h.X}
	q1 := vec3{a0.Z, a0.W, h.Y}
	q2 := vec3{a1.X, a1.Y, h.Z}
	q3 := vec3{a1.Z, a1.W, h.W}

	// A zero length vector would make the inverse square root infinite and
	// the final sum NaN; such a corner simply contributes nothing.
	invSqrt := func(v vec3) float64 {
		length := v.dot(v)
		if length <= 1e-12 {
			return 0
		}
		return 1 / math.Sqrt(length)
	}
	q0 = q0.mul(invSqrt(q0))
	q1 = q1.mul(invSqrt(q1))
	q2 = q2.mul(invSqrt(q2))
	q3 = q3.mul(invSqrt(q3))

	m := vec4{
		math.Max(0.6-x0.dot(x0), 0),
		math.Max(0.6-x1.dot(x1), 0),
		math.Max(0.6-x2.dot(x2), 0),
		math.Max(0.6-x3.dot(x3), 0),
	}
	m = vec4{m.X * m.X, m.Y * m.Y, m.Z * m.Z, m.W * m.W}

	sum := m.X*m.X*q0.dot(x0) + m.Y*m.Y*q1.dot(x1) +
		m.Z*m.Z*q2.dot(x2) + m.W*m.W*q3.dot(x3)
	if math.IsNaN(sum) || math.IsInf(sum, 0) {
		return 0
	}
	return 42 * sum
}

func (v vec4) addVec4(o vec4) vec4 {
	return vec4{v.X + o.X, v.Y + o.Y, v.Z + o.Z, v.W + o.W}
}

func triWave(x float64) float64 { return math.Abs(fract(x) - 0.5) }

func triVec(p vec3) vec3 {
	return vec3{
		triWave(p.Z + triWave(p.Y*20)),
		triWave(p.Z + triWave(p.X)),
		triWave(p.Y + triWave(p.X)),
	}
}

func triField3d(seed vec3, phase, spd float64) float64 {
	z := 0.4
	acc := 0.1
	p0 := seed
	base := seed

	for i := 0; i < 4; i++ {
		g := triVec(base.mul(0.01))
		add := phase * 0.1 * spd
		p0 = p0.add(vec3{g.X + add, g.Y + add, g.Z + add})
		base = base.mul(4)
		z *= 0.9
		p0 = p0.mul(1.6)
		acc += triWave(p0.Z+triWave(0.6*p0.X+0.1*triWave(p0.Y))) / z
	}

	s := acc + math.Sin(acc+math.Sin(z)*2.8)*2.2
	return smoothstep(0, 8, s)
}

func spin2d(v vec2, angle float64) vec2 {
	s, c := math.Sin(angle), math.Cos(angle)
	return vec2{v.X*c - v.Y*s, v.X*s + v.Y*c}
}

func falloff(intensity, atten, dist float64) float64 {
	return intensity / (1 + dist + dist*atten)
}

func shapeNoiseLobe(uv vec2, nVal float64, c1, c2 vec3, strength, phase, ofs float64) (vec3, float64) {
	lenUV := math.Hypot(uv.X, uv.Y)
	r0 := nVal
	safe := math.Max(lenUV, 1e-4)
	onRing := vec2{uv.X * r0 / safe, uv.Y * r0 / safe}
	radial := math.Hypot(uv.X-onRing.X, uv.Y-onRing.Y)

	edge := r0 + 0.1 + (math.Sin(phase+ofs) + 1)
	mask := smoothstep(edge, r0, lenUV)

	lightPower := 0.07*(1+1.5*(-math.Sin(phase*2+ofs*0.5)*0.5)) + 0.15*strength
	glow := falloff(lightPower, 10, radial)

	t := clamp(uv.Y*2, 0, 1)
	col := vec3{
		c1.X + (c2.X-c1.X)*t,
		c1.Y + (c2.Y-c1.Y)*t,
		c1.Z + (c2.Z-c1.Z)*t,
	}
	col = vec3{
		clamp(col.X+col.X*glow, 0, 1),
		clamp(col.Y+col.Y*glow, 0, 1),
		clamp(col.Z+col.Z*glow, 0, 1),
	}
	return col, mask
}

func composeRing(uv vec2, nVal, baseR float64, c1, c2 vec3, width, spark, likeVal, audioVal, phase, ofs float64, noisePos vec2) (vec3, float64) {
	lenUV := math.Hypot(uv.X, uv.Y)
	gain := math.Max(likeVal, audioVal*0.6)
	outer := baseR + width*0.5 + spark*(1+gain*50*spark)
	strength := math.Max(likeVal, audioVal)

	blobUV := uv.mul(1 - likeVal*0.5).add(noisePos)
	blob, alpha := shapeNoiseLobe(blobUV, nVal, c1, c2, strength, phase, ofs)

	alphaScale := smoothstep(outer, 0.5, lenUV)
	alpha *= alphaScale

	halo := 0.25 * likeVal * (1 - smoothstep(0.2, outer*0.8, lenUV))
	return vec3{
		blob.X + c1.X*halo,
		blob.Y + c1.Y*halo,
		blob.Z + c1.Z*halo,
	}, alpha
}

// getUV maps a half cell to the normalized coordinates of the shader.
func getUV(x, y int, yOffset, width, height, zoom float64) vec2 {
	yNorm := (float64(y)*2 + yOffset) / (height * 2)
	xNorm := (float64(x) + 0.5) / width
	uv := vec2{xNorm*2 - 1, yNorm*2 - 1}
	minDim := math.Min(width, height*2)
	sx := width / minDim / zoom
	sy := (height * 2) / minDim / zoom
	return vec2{uv.X * sx, -uv.Y * sy}
}

var orbits = [3][3]float64{
	{0.5, 0.5, 0.2},
	{0.2, 0.8, -0.3},
	{0.8, 0.2, 0.4},
}

// getPixelColor is the shader's get_pixel_color.
func getPixelColor(uv vec2, state animState) vec3 {
	spark := calculateSpark(uv, state.phase)

	n0 := simplex3(vec3{uv.X * 1.2, uv.Y * 1.2, state.phase * 0.5})

	color := vec3{state.bg[0], state.bg[1], state.bg[2]}
	level := state.amplitude
	response := [3]float64{level * 1.2, level, level * 0.8}
	spectrum := [3]float64{level, level * 0.8, level * 0.6}

	for i := 0; i < 3; i++ {
		off := 1.57 * float64(i)
		react := response[i]
		rot := orbits[i]
		noisePos := spin2d(vec2{rot[0], rot[1]}, state.phase*rot[2])
		transformed := uv.mul(1 - react*0.5).add(noisePos)
		bandNoise := simplex3(vec3{
			transformed.X*1.2 + off,
			transformed.Y*1.2 + off,
			state.phase*0.5 + off,
		})*0.5 + 0.5

		radius := 0.95 - 0.15*float64(i)
		c1 := vec3{state.palette[i][0], state.palette[i][1], state.palette[i][2]}
		c2 := vec3{state.palette[i+3][0], state.palette[i+3][1], state.palette[i+3][2]}
		mixedR := radius + 0.3*n0
		wRing := 0.8 - 0.2*float64(i)
		sparkBase := (1 - 0.3*float64(i)) * spark

		blob, w := composeRing(uv, bandNoise, mixedR, c1, c2, wRing, sparkBase,
			react, spectrum[i], state.phase, off, noisePos)
		color = vec3{
			color.X + (blob.X-color.X)*w,
			color.Y + (blob.Y-color.Y)*w,
			color.Z + (blob.Z-color.Z)*w,
		}
	}

	if state.glow > 0 {
		brightness := state.glow * 1.1
		boost := vec3{
			mix(state.palette[0][0], 1, 0.4),
			mix(state.palette[0][1], 1, 0.4),
			mix(state.palette[0][2], 1, 0.4),
		}
		radial := clamp(math.Hypot(uv.X, uv.Y), 0, 1.5)
		fade := math.Exp(-radial * radial * 1.5)
		color = vec3{
			color.X + boost.X*brightness*fade,
			color.Y + boost.Y*brightness*fade,
			color.Z + boost.Z*brightness*fade,
		}
	}

	return vec3{clamp(color.X, 0, 1), clamp(color.Y, 0, 1), clamp(color.Z, 0, 1)}
}

func calculateSpark(uv vec2, phase float64) float64 {
	lenUV := math.Hypot(uv.X, uv.Y)
	dir := vec2{1, 0}
	if lenUV > 0.0001 {
		dir = vec2{uv.X / lenUV, uv.Y / lenUV}
	}
	seed := vec3{dir.X * 0.16, dir.Y * 0.16, 0}

	spark := triField3d(seed, phase, 0.1)
	s2 := spark * spark
	s10 := (s2 * s2) * (s2 * s2) * s2
	spark = spark*0.2 + s10

	return smoothstep(0, math.Max(spark, 1e-5), 0.3) * spark
}

func renderFrame(width, height int, state animState) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Preallocate: every cell may need two 24 bit colours, so a generous
	// estimate avoids most reallocations of the hot path.
	var b []byte
	b = make([]byte, 0, width*height*64)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			top := getPixelColor(getUV(x, y, 0.5, float64(width), float64(height), 0.8), state)
			bottom := getPixelColor(getUV(x, y, 1.5, float64(width), float64(height), 0.8), state)

			// One SGR sequence sets the foreground of the top half and
			// the background of the bottom half; the reset is emitted once
			// per line instead of once per cell.
			b = append(b, "\x1b[38;2;"...)
			b = strconv.AppendInt(b, int64(top.X*255+0.5), 10)
			b = append(b, ';')
			b = strconv.AppendInt(b, int64(top.Y*255+0.5), 10)
			b = append(b, ';')
			b = strconv.AppendInt(b, int64(top.Z*255+0.5), 10)
			b = append(b, ";48;2;"...)
			b = strconv.AppendInt(b, int64(bottom.X*255+0.5), 10)
			b = append(b, ';')
			b = strconv.AppendInt(b, int64(bottom.Y*255+0.5), 10)
			b = append(b, ';')
			b = strconv.AppendInt(b, int64(bottom.Z*255+0.5), 10)
			b = append(b, "m▀"...)
		}
		b = append(b, "\x1b[0m"...)
		if y != height-1 {
			b = append(b, '\n')
		}
	}

	return string(b)
}
