package tts

import (
	"math"
)

// The voice is Pico's, made deeper and male: Pico speaks 1/0.7 faster and 25%
// lower, and its 16 kHz output is played as if it were 11.2 kHz, which lowers
// pitch and formants together at normal tempo. Then a clarity chain makes up
// for the dullness (the "k70p75 eqc" preset picked by ear).
const (
	playbackRate = 11200
	prosody      = "<speed level='142'><pitch level='75'>%s</pitch></speed>"

	highpassHz    = 90
	mudHz         = 300
	mudDB         = -3
	presenceHz    = 2500
	presenceDB    = 5
	compRatio     = 3
	compThreshold = 0.25
	compAttack    = 0.005
	compRelease   = 0.08
	outPeak       = 0.9

	lanczosA = 16
)

// clarify runs the clarity chain over Pico PCM, at playbackRate.
func clarify(pcm []int16) []float32 {
	x := make([]float32, len(pcm))
	for i, v := range pcm {
		x[i] = float32(v) / 32768
	}
	normalize(x, 1)
	highpass(highpassHz, playbackRate).apply(x)
	peaking(mudHz, mudDB, 1, playbackRate).apply(x)
	peaking(presenceHz, presenceDB, 1, playbackRate).apply(x)
	normalize(x, 1)
	compress(x, playbackRate)
	normalize(x, outPeak)
	return x
}

func normalize(x []float32, peak float32) {
	var m float32
	for _, v := range x {
		m = max(m, float32(math.Abs(float64(v))))
	}
	if m == 0 {
		return
	}
	for i := range x {
		x[i] *= peak / m
	}
}

// biquad is an RBJ audio-EQ-cookbook filter, normalized by a0.
type biquad struct{ b0, b1, b2, a1, a2 float64 }

func highpass(fc, rate float64) biquad {
	w := 2 * math.Pi * fc / rate
	// Q = 1/√2, Butterworth
	cw, al := math.Cos(w), math.Sin(w)/math.Sqrt2
	a0 := 1 + al
	return biquad{(1 + cw) / 2 / a0, -(1 + cw) / a0, (1 + cw) / 2 / a0, -2 * cw / a0, (1 - al) / a0}
}

func peaking(fc, db, q, rate float64) biquad {
	a := math.Pow(10, db/40)
	w := 2 * math.Pi * fc / rate
	cw, al := math.Cos(w), math.Sin(w)/(2*q)
	a0 := 1 + al/a
	return biquad{(1 + al*a) / a0, -2 * cw / a0, (1 - al*a) / a0, -2 * cw / a0, (1 - al/a) / a0}
}

func (f biquad) apply(x []float32) {
	var x1, x2, y1, y2 float64
	for i, v := range x {
		in := float64(v)
		y := f.b0*in + f.b1*x1 + f.b2*x2 - f.a1*y1 - f.a2*y2
		x2, x1, y2, y1 = x1, in, y1, y
		x[i] = float32(y)
	}
}

// compress evens out loud vowels against quiet consonants so the latter carry.
func compress(x []float32, rate float64) {
	att := float32(math.Exp(-1 / (compAttack * rate)))
	rel := float32(math.Exp(-1 / (compRelease * rate)))
	var env float32
	for i, v := range x {
		a := float32(math.Abs(float64(v)))
		k := rel
		if a > env {
			k = att
		}
		env = k*env + (1-k)*a
		if env > compThreshold {
			x[i] *= compThreshold * float32(math.Pow(float64(env/compThreshold), 1.0/compRatio)) / env
		}
	}
}

// resample upsamples with a Lanczos (windowed sinc) kernel and converts to
// 16-bit PCM; to must not be below from, as there is no anti-alias filter.
func resample(x []float32, from, to int) []int16 {
	step := float64(from) / float64(to)
	out := make([]int16, int(float64(len(x))/step))
	for j := range out {
		t := float64(j) * step
		c := int(math.Floor(t))
		var s float64
		for k := max(c-lanczosA+1, 0); k <= min(c+lanczosA, len(x)-1); k++ {
			s += float64(x[k]) * lanczos(t-float64(k))
		}
		out[j] = int16(math.Round(max(-1, min(1, s)) * math.MaxInt16))
	}
	return out
}

func lanczos(x float64) float64 {
	if x == 0 {
		return 1
	}
	if math.Abs(x) >= lanczosA {
		return 0
	}
	px := math.Pi * x
	return lanczosA * math.Sin(px) * math.Sin(px/lanczosA) / (px * px)
}
