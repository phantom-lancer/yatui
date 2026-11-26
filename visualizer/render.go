package visualizer

import (
	"math"
	"strconv"
)

// The renderer evaluates the noise field on a coarse grid and interpolates it
// for the terminal cells. The rings are soft blobs, so sampling every second
// or third cell is invisible while the cost drops four to nine times. This is
// the single biggest saving: the noise is the only expensive part of a frame.
const (
	// sparkBuckets is the size of the per-frame sparkle table. The sparkle
	// depends only on the direction from the centre, so one table replaces
	// one triField3d evaluation per pixel.
	sparkBuckets = 128
)

// sample is one half cell: where it looks and which sparkle bucket it uses.
type sample struct {
	ux, uy float64
	bucket uint16
}

// layout is the geometry of a frame. It only changes on resize, so the
// per-pixel trigonometry is computed once and reused by every frame.
type layout struct {
	width, height int
	detail        int
	gridW, gridH  int
	samples       []sample
	// gridUV and gridBucket are the geometry of the coarse grid. They never
	// change between frames, so the per-pixel trigonometry and the direction
	// lookup run once per resize.
	gridUV     []vec2
	gridBucket []uint16
}

func newLayout(width, height, detail int) *layout {
	if width <= 0 || height <= 0 {
		return &layout{}
	}
	if detail < 1 {
		detail = 1
	}

	l := &layout{
		width:  width,
		height: height,
		detail: detail,
	}
	l.gridW = (width + detail - 1) / detail
	l.gridH = (height*2 + detail - 1) / detail
	l.samples = make([]sample, width*height*2)
	l.gridUV = make([]vec2, l.gridW*l.gridH)
	l.gridBucket = make([]uint16, l.gridW*l.gridH)

	for gy := 0; gy < l.gridH; gy++ {
		for gx := 0; gx < l.gridW; gx++ {
			uv := l.gridUVAt(gx, gy)
			l.gridUV[gy*l.gridW+gx] = uv
			l.gridBucket[gy*l.gridW+gx] = bucketFor(uv)
		}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			top := getUV(x, y, 0.5, float64(width), float64(height), 0.8)
			bottom := getUV(x, y, 1.5, float64(width), float64(height), 0.8)
			l.samples[(y*width+x)*2] = sample{ux: top.X, uy: top.Y, bucket: bucketFor(top)}
			l.samples[(y*width+x)*2+1] = sample{ux: bottom.X, uy: bottom.Y, bucket: bucketFor(bottom)}
		}
	}
	return l
}

// bucketFor maps a direction to the sparkle table.
func bucketFor(uv vec2) uint16 {
	length := math.Sqrt(uv.X*uv.X + uv.Y*uv.Y)
	if length < 1e-6 {
		return 0
	}
	angle := math.Atan2(uv.Y, uv.X)
	if angle < 0 {
		angle += 2 * math.Pi
	}
	index := int(angle / (2 * math.Pi) * sparkBuckets)
	if index >= sparkBuckets {
		index = sparkBuckets - 1
	}
	return uint16(index)
}

// scene is everything that changes between frames.
type scene struct {
	phase     float64
	amplitude float64
	glow      float64
	palette   [6][3]float64
	bg        [3]float64
	spark     [sparkBuckets]float64
	// noisePos is the orbit offset of every ring, constant for a frame.
	noisePos [3]vec2
}

// prepare fills the per-frame tables: the sparkle values and the orbit
// offsets. Both used to be recomputed for every pixel.
func (s *scene) prepare() {
	for i := 0; i < 3; i++ {
		rot := orbits[i]
		s.noisePos[i] = spin2d(vec2{rot[0], rot[1]}, s.phase*rot[2])
	}
	s.buildSpark()
}

// buildSpark fills the sparkle table for the current phase.
func (s *scene) buildSpark() {
	for i := 0; i < sparkBuckets; i++ {
		angle := 2 * math.Pi * float64(i) / sparkBuckets
		s.spark[i] = sparkleFor(math.Cos(angle), math.Sin(angle), s.phase)
	}
}

// sparkleAt reads the table between two buckets.
func (s *scene) sparkleAt(bucket uint16) float64 {
	index := int(bucket)
	next := index + 1
	if next >= sparkBuckets {
		next = 0
	}
	// The table is sampled uniformly in angle; a linear blend hides the
	// steps of a 160 entry table without extra noise evaluations.
	f := float64(bucket) - float64(index)
	return s.spark[index] + (s.spark[next]-s.spark[index])*f
}

// sparkleFor is calculate_spark of the shader for one direction.
func sparkleFor(dx, dy, phase float64) float64 {
	seed := vec3{dx * 0.16, dy * 0.16, 0}
	spark := triField3d(seed, phase, 0.1)
	s2 := spark * spark
	s10 := (s2 * s2) * (s2 * s2) * s2
	spark = spark*0.2 + s10
	return smoothstep(0, math.Max(spark, 1e-5), 0.3) * spark
}

// buffer holds the working memory of one frame. It is owned by the renderer
// worker and reused, so a steady animation does not allocate per frame.
type buffer struct {
	layout   *layout
	colors   []vec3
	cells    []RGB
	top      []RGB
	bottom   []RGB
	text     []byte
	cellText []byte
}

func newBuffer() *buffer { return &buffer{} }

// prepare makes the buffer match the layout.
func (b *buffer) prepare(l *layout) {
	if b.layout == l {
		return
	}
	b.layout = l
	b.colors = make([]vec3, l.gridW*l.gridH)
	b.cells = make([]RGB, l.width*l.height)
	b.top = make([]RGB, l.width*l.height)
	b.bottom = make([]RGB, l.width*l.height)
	if cap(b.text) < l.width*l.height*48 {
		b.text = make([]byte, 0, l.width*l.height*48)
	}
	b.cellText = b.cellText[:0]
}

// gridUVAt computes the uv of a grid point.
func (l *layout) gridUVAt(gx, gy int) vec2 {
	x := gx * l.detail
	y := gy * l.detail
	if x >= l.width {
		x = l.width - 1
	}
	// y is a sample index: two samples per cell
	if y >= l.height*2 {
		y = l.height*2 - 1
	}
	cell := y / 2
	offset := 1.5
	if y%2 == 0 {
		offset = 0.5
	}
	return getUV(x, cell, offset, float64(l.width), float64(l.height), 0.8)
}

// render evaluates the scene into the buffer: the average colour of every
// cell and, when text is wanted, the half block frame.
func (b *buffer) render(s *scene, wantText bool) ([]RGB, string) {
	l := b.layout
	if l == nil || l.width <= 0 || l.height <= 0 {
		return nil, ""
	}

	// The coarse grid: one noise evaluation per grid point. The geometry and
	// the sparkle bucket of every point are precomputed by the layout.
	for i := range b.colors {
		uv := l.gridUV[i]
		b.colors[i] = getPixelColorSpark(uv, s, s.sparkleAt(l.gridBucket[i]))
	}

	// Bilinear upsample to the sample grid, then to the cells.
	for cy := 0; cy < l.height; cy++ {
		for cx := 0; cx < l.width; cx++ {
			top := b.sample(cx, cy, 0)
			bottom := b.sample(cx, cy, 1)
			b.top[cy*l.width+cx] = toRGB(top)
			b.bottom[cy*l.width+cx] = toRGB(bottom)
			b.cells[cy*l.width+cx] = toRGB(vec3{
				X: (top.X + bottom.X) * 0.5,
				Y: (top.Y + bottom.Y) * 0.5,
				Z: (top.Z + bottom.Z) * 0.5,
			})
		}
	}

	if !wantText {
		return b.cells, ""
	}
	return b.cells, b.renderText()
}

// sample interpolates one half cell from the coarse grid.
func (b *buffer) sample(x, y, half int) vec3 {
	l := b.layout
	fx := float64(x) / float64(l.detail)
	fy := float64(y*2+half) / float64(l.detail)

	gx0 := int(fx)
	gy0 := int(fy)
	if gx0 >= l.gridW {
		gx0 = l.gridW - 1
	}
	if gy0 >= l.gridH {
		gy0 = l.gridH - 1
	}
	gx1 := gx0 + 1
	gy1 := gy0 + 1
	if gx1 >= l.gridW {
		gx1 = gx0
	}
	if gy1 >= l.gridH {
		gy1 = gy0
	}

	tx := fx - float64(gx0)
	ty := fy - float64(gy0)

	c00 := b.colors[gy0*l.gridW+gx0]
	c10 := b.colors[gy0*l.gridW+gx1]
	c01 := b.colors[gy1*l.gridW+gx0]
	c11 := b.colors[gy1*l.gridW+gx1]

	top := vec3{
		X: c00.X + (c10.X-c00.X)*tx,
		Y: c00.Y + (c10.Y-c00.Y)*tx,
		Z: c00.Z + (c10.Z-c00.Z)*tx,
	}
	bottom := vec3{
		X: c01.X + (c11.X-c01.X)*tx,
		Y: c01.Y + (c11.Y-c01.Y)*tx,
		Z: c01.Z + (c11.Z-c01.Z)*tx,
	}
	return vec3{
		X: top.X + (bottom.X-top.X)*ty,
		Y: top.Y + (bottom.Y-top.Y)*ty,
		Z: top.Z + (bottom.Z-top.Z)*ty,
	}
}

func toRGB(c vec3) RGB {
	return RGB{
		R: uint8(clamp(c.X, 0, 1)*255 + 0.5),
		G: uint8(clamp(c.Y, 0, 1)*255 + 0.5),
		B: uint8(clamp(c.Z, 0, 1)*255 + 0.5),
	}
}

// renderText builds the half block frame: the upper half block with the top
// colour as the foreground and the bottom colour as the background.
func (b *buffer) renderText() string {
	l := b.layout
	out := b.text[:0]

	for y := 0; y < l.height; y++ {
		for x := 0; x < l.width; x++ {
			top := b.top[y*l.width+x]
			bottom := b.bottom[y*l.width+x]
			out = append(out, "\x1b[38;2;"...)
			out = appendByte3(out, top.R, top.G, top.B)
			out = append(out, ";48;2;"...)
			out = appendByte3(out, bottom.R, bottom.G, bottom.B)
			out = append(out, "m▀"...)
		}
		out = append(out, "\x1b[0m"...)
		if y != l.height-1 {
			out = append(out, '\n')
		}
	}

	b.text = out
	return string(out)
}

// appendByte3 appends "r;g;b" without allocations or strconv calls.
func appendByte3(dst []byte, r, g, b uint8) []byte {
	dst = appendUint8(dst, r)
	dst = append(dst, ';')
	dst = appendUint8(dst, g)
	dst = append(dst, ';')
	return appendUint8(dst, b)
}

var digitPairs = [200]byte{
	'0', '0', '0', '1', '0', '2', '0', '3', '0', '4', '0', '5', '0', '6', '0', '7', '0', '8', '0', '9',
	'1', '0', '1', '1', '1', '2', '1', '3', '1', '4', '1', '5', '1', '6', '1', '7', '1', '8', '1', '9',
	'2', '0', '2', '1', '2', '2', '2', '3', '2', '4', '2', '5', '2', '6', '2', '7', '2', '8', '2', '9',
	'3', '0', '3', '1', '3', '2', '3', '3', '3', '4', '3', '5', '3', '6', '3', '7', '3', '8', '3', '9',
	'4', '0', '4', '1', '4', '2', '4', '3', '4', '4', '4', '5', '4', '6', '4', '7', '4', '8', '4', '9',
	'5', '0', '5', '1', '5', '2', '5', '3', '5', '4', '5', '5', '5', '6', '5', '7', '5', '8', '5', '9',
	'6', '0', '6', '1', '6', '2', '6', '3', '6', '4', '6', '5', '6', '6', '6', '7', '6', '8', '6', '9',
	'7', '0', '7', '1', '7', '2', '7', '3', '7', '4', '7', '5', '7', '6', '7', '7', '7', '8', '7', '9',
	'8', '0', '8', '1', '8', '2', '8', '3', '8', '4', '8', '5', '8', '6', '8', '7', '8', '8', '8', '9',
	'9', '0', '9', '1', '9', '2', '9', '3', '9', '4', '9', '5', '9', '6', '9', '7', '9', '8', '9', '9',
}

func appendUint8(dst []byte, v uint8) []byte {
	if v < 100 {
		return append(dst, digitPairs[v*2], digitPairs[v*2+1])
	}
	return strconv.AppendUint(dst, uint64(v), 10)
}

// getPixelColorSpark is get_pixel_color with the sparkle taken from the
// precomputed table.
func getPixelColorSpark(uv vec2, s *scene, spark float64) vec3 {
	n0 := simplex3(vec3{uv.X * 1.2, uv.Y * 1.2, s.phase * 0.5})

	color := vec3{s.bg[0], s.bg[1], s.bg[2]}
	level := s.amplitude
	response := [3]float64{level * 1.2, level, level * 0.8}
	spectrum := [3]float64{level, level * 0.8, level * 0.6}

	for i := 0; i < 3; i++ {
		off := 1.57 * float64(i)
		react := response[i]
		noisePos := s.noisePos[i]
		transformed := uv.mul(1 - react*0.5).add(noisePos)
		bandNoise := simplex3(vec3{
			transformed.X*1.2 + off,
			transformed.Y*1.2 + off,
			s.phase*0.5 + off,
		})*0.5 + 0.5

		radius := 0.95 - 0.15*float64(i)
		c1 := vec3{s.palette[i][0], s.palette[i][1], s.palette[i][2]}
		c2 := vec3{s.palette[i+3][0], s.palette[i+3][1], s.palette[i+3][2]}
		mixedR := radius + 0.3*n0
		wRing := 0.8 - 0.2*float64(i)
		sparkBase := (1 - 0.3*float64(i)) * spark

		blob, w := composeRing(uv, bandNoise, mixedR, c1, c2, wRing, sparkBase,
			react, spectrum[i], s.phase, off, noisePos)
		color = vec3{
			color.X + (blob.X-color.X)*w,
			color.Y + (blob.Y-color.Y)*w,
			color.Z + (blob.Z-color.Z)*w,
		}
	}

	if s.glow > 0 {
		brightness := s.glow * 1.1
		boost := vec3{
			mix(s.palette[0][0], 1, 0.4),
			mix(s.palette[0][1], 1, 0.4),
			mix(s.palette[0][2], 1, 0.4),
		}
		radial := clamp(math.Sqrt(uv.X*uv.X+uv.Y*uv.Y), 0, 1.5)
		fade := math.Exp(-radial * radial * 1.5)
		color = vec3{
			color.X + boost.X*brightness*fade,
			color.Y + boost.Y*brightness*fade,
			color.Z + boost.Z*brightness*fade,
		}
	}

	return vec3{clamp(color.X, 0, 1), clamp(color.Y, 0, 1), clamp(color.Z, 0, 1)}
}
