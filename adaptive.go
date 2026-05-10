package main

import (
	"image"
	"time"
)

const (
	// adjustWindow is how many frames we accumulate before re-evaluating FPS.
	adjustWindow = 20
	// historySize is the max number of "new-frame" timestamps to keep for
	// estimating the animation's native FPS.
	historySize = 40
)

// AdaptiveCapture tracks how fast the cimbar animation is running and adjusts
// the screen-capture interval to match — staying at 1.5× the animation rate
// so we never miss a unique frame.
//
// Two operating modes:
//   - Idle  (cimbar not yet visible): fixed at cfg.MinFPS to save CPU.
//   - Active (cimbar on screen):      adaptive between cfg.MinActiveFPS and cfg.MaxFPS.
type AdaptiveCapture struct {
	// current capture rate
	currentFPS float64

	// configurable bounds
	minIdleFPS   float64
	minActiveFPS float64
	maxFPS       float64

	// state
	active bool

	// per-window counters (reset after adjustWindow frames)
	windowTotal int
	windowNew   int

	// frame identity
	lastHash uint64

	// timestamps of recent "new frame" detections, used to estimate anim FPS
	newFrameTimes []time.Time

	log *Logger
}

// NewAdaptiveCapture creates a controller with the given FPS bounds.
func NewAdaptiveCapture(minIdle, minActive, maxFPS float64, log *Logger) *AdaptiveCapture {
	return &AdaptiveCapture{
		currentFPS:    minIdle,
		minIdleFPS:    minIdle,
		minActiveFPS:  minActive,
		maxFPS:        maxFPS,
		newFrameTimes: make([]time.Time, 0, historySize),
		log:           log,
	}
}

// Interval returns the current sleep duration between captures.
func (a *AdaptiveCapture) Interval() time.Duration {
	fps := a.currentFPS
	if fps < 0.1 {
		fps = 0.1
	}
	return time.Duration(float64(time.Second) / fps)
}

// CurrentFPS returns the current capture rate.
func (a *AdaptiveCapture) CurrentFPS() float64 { return a.currentFPS }

// SetActive switches between idle and active mode.
// Switching to active resets the learning window and bumps FPS to minActiveFPS.
func (a *AdaptiveCapture) SetActive(active bool) {
	if active == a.active {
		return
	}
	a.active = active
	if active {
		// Jump immediately to the minimum active rate; learning will increase it.
		a.currentFPS = a.minActiveFPS
		a.windowTotal = 0
		a.windowNew = 0
		a.newFrameTimes = a.newFrameTimes[:0]
		a.log.Info("adaptive: switched to active mode (%.1f fps)", a.currentFPS)
	} else {
		a.currentFPS = a.minIdleFPS
		a.log.Info("adaptive: switched to idle mode (%.1f fps)", a.currentFPS)
	}
}

// Observe records a freshly captured frame and returns true if the frame is
// visually different from the previous one (i.e. a new animation frame).
// It also triggers an FPS adjustment every adjustWindow frames.
func (a *AdaptiveCapture) Observe(img image.Image) bool {
	h := frameHash(img)
	isNew := h != a.lastHash
	a.lastHash = h

	if isNew {
		a.windowNew++
		now := time.Now()
		a.newFrameTimes = append(a.newFrameTimes, now)
		if len(a.newFrameTimes) > historySize {
			// Drop oldest, keep the ring at historySize.
			copy(a.newFrameTimes, a.newFrameTimes[1:])
			a.newFrameTimes = a.newFrameTimes[:historySize]
		}
	}
	a.windowTotal++

	if a.active && a.windowTotal >= adjustWindow {
		a.adjust()
		a.windowTotal = 0
		a.windowNew = 0
	}

	return isNew
}

// adjust recalculates currentFPS based on the observed animation rate.
func (a *AdaptiveCapture) adjust() {
	if a.windowTotal == 0 {
		return
	}

	newRatio := float64(a.windowNew) / float64(a.windowTotal)
	animFPS := a.estimateAnimFPS()
	oldFPS := a.currentFPS

	if animFPS > 0 {
		// Target: 1.5× the animation's native rate (Nyquist with margin).
		target := clampFPS(animFPS*1.5, a.minActiveFPS, a.maxFPS)
		// Exponential moving average: move 40% of the way toward target.
		a.currentFPS = a.currentFPS*0.6 + target*0.4
	} else {
		// No history yet: adjust directionally based on new/duplicate ratio.
		switch {
		case newRatio > 0.75:
			// Almost every frame is new → we are too slow, missing frames.
			a.currentFPS = clampFPS(a.currentFPS*1.3, a.minActiveFPS, a.maxFPS)
		case newRatio < 0.25:
			// Mostly duplicates → we are too fast, wasting CPU.
			a.currentFPS = clampFPS(a.currentFPS*0.8, a.minActiveFPS, a.maxFPS)
		}
	}

	if absF(a.currentFPS-oldFPS) >= 0.3 {
		a.log.Info("adaptive: FPS %.1f → %.1f  (new %d/%d = %.0f%%, anim ≈ %.1f fps)",
			oldFPS, a.currentFPS,
			a.windowNew, a.windowTotal, newRatio*100,
			animFPS)
	} else {
		a.log.Debug("adaptive: FPS stable at %.1f  (new %.0f%%, anim ≈ %.1f fps)",
			a.currentFPS, newRatio*100, animFPS)
	}
}

// estimateAnimFPS estimates the animation's native frame rate from the
// timestamps of recently observed unique frames.
func (a *AdaptiveCapture) estimateAnimFPS() float64 {
	n := len(a.newFrameTimes)
	if n < 3 {
		return 0
	}
	span := a.newFrameTimes[n-1].Sub(a.newFrameTimes[0]).Seconds()
	if span <= 0 {
		return 0
	}
	return float64(n-1) / span
}

// ─── frame hashing ───────────────────────────────────────────────────────────

// frameHash computes a fast perceptual hash by sampling an 8×8 grid of pixels.
// Two frames with the same hash are considered visually identical for our
// purposes (we do not need cryptographic strength here).
func frameHash(img image.Image) uint64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	const grid = 8
	var hash uint64
	for gy := 0; gy < grid; gy++ {
		for gx := 0; gx < grid; gx++ {
			// Sample the centre of each grid cell.
			px := b.Min.X + gx*w/grid + w/(grid*2)
			py := b.Min.Y + gy*h/grid + h/(grid*2)
			r, g, bv, _ := img.At(px, py).RGBA()
			// Fibonacci-hashing mix to spread bits evenly.
			hash ^= uint64(r>>8)*2654435761 + uint64(g>>8)*40503 + uint64(bv>>8)*12345
			hash = hash<<13 | hash>>(64-13) // rotate left 13
		}
	}
	return hash
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func clampFPS(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
