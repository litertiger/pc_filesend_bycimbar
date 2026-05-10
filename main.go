// cimbar-recv-pc — PC-side receiver for cimbar encoded data
//
// Usage:
//
//	cimbar-recv-pc.exe [flags]
//
// The program captures the local screen, detects when a cimbar code is being
// displayed (e.g. from https://cimbar.org in a browser), decodes it using the
// external cimbar_recv binary, and saves the resulting file.
//
// Flags:
//
//	-output     Output directory (default: %USERPROFILE%\Downloads on Windows, . elsewhere)
//	-timeout    How long to wait before giving up if no code appears (default: 20s)
//	-min-fps    Minimum capture rate while scanning for a code (default: 2)
//	-max-fps    Maximum capture rate during active decoding (default: 30)
//	-display    Display index (default: 0)
//	-cimbar     Path to cimbar_recv binary (default: cimbar_recv from PATH)
//	-region     Capture region WxH+X+Y, or "auto" for full display (default: auto)
//	-verbose    Print cimbar_recv output (default: false)
//	-keep       Keep captured frames after decoding (default: false)
//	-log        Log file path (default: empty = console only)
//	-loglevel   Log verbosity: debug | info | warn | error (default: info)
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	cfg := parseFlags()

	// ── Set up logging ────────────────────────────────────────────────────────
	level := ParseLogLevel(cfg.LogLevel)
	var log *Logger
	if cfg.LogFile != "" {
		logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: cannot open log file:", err)
			log = NewLogger(level, os.Stderr)
		} else {
			defer logFile.Close()
			log = NewLogger(level, os.Stderr, logFile)
		}
	} else {
		log = NewLogger(level, os.Stderr)
	}

	// ── Banner ────────────────────────────────────────────────────────────────
	log.Info("=== cimbar PC Receiver ===")
	log.Info("output dir    : %s", cfg.OutputDir)
	log.Info("timeout       : %s", cfg.DetectTimeout)
	log.Info("FPS range     : %.0f – %.0f (adaptive)", cfg.MinFPS, cfg.MaxFPS)
	log.Info("display       : %d", cfg.Display)
	log.Info("cimbar_recv   : %s", cfg.CimbarBin)
	if cfg.LogFile != "" {
		log.Info("log file      : %s", cfg.LogFile)
	}

	// ── Prerequisite checks ───────────────────────────────────────────────────
	if err := checkCimbarBin(cfg.CimbarBin); err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}

	display, err := getDisplay(cfg.Display)
	if err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}

	region, err := parseRegion(cfg.Region, display)
	if err != nil {
		log.Error("%v", err)
		os.Exit(1)
	}
	log.Info("capture region: %dx%d at (%d,%d)",
		region.Dx(), region.Dy(), region.Min.X, region.Min.Y)

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		log.Error("cannot create output dir: %v", err)
		os.Exit(1)
	}

	// ── Run ───────────────────────────────────────────────────────────────────
	r := &Receiver{cfg: cfg, region: region, log: log}
	if err := r.Run(); err != nil {
		log.Error("%v", err)
		notifyError("cimbar receiver failed", err.Error())
		termBell()
		os.Exit(1)
	}
}

// ─── Config ──────────────────────────────────────────────────────────────────

// Config holds all runtime parameters.
type Config struct {
	OutputDir     string
	DetectTimeout time.Duration
	MinFPS        float64
	MaxFPS        float64
	Display       int
	CimbarBin     string
	Region        string
	Verbose       bool
	KeepFrames    bool
	LogFile       string
	LogLevel      string
}

func parseFlags() Config {
	var cfg Config
	flag.StringVar(&cfg.OutputDir, "output", defaultOutputDir(),
		"output directory for received files")
	flag.DurationVar(&cfg.DetectTimeout, "timeout", 20*time.Second,
		"timeout if no cimbar code detected on screen")
	flag.Float64Var(&cfg.MinFPS, "min-fps", 2,
		"minimum capture FPS while waiting for cimbar code to appear")
	flag.Float64Var(&cfg.MaxFPS, "max-fps", 30,
		"maximum capture FPS during active decoding")
	flag.IntVar(&cfg.Display, "display", 0, "display index to monitor")
	flag.StringVar(&cfg.CimbarBin, "cimbar", "cimbar_recv",
		"path to cimbar_recv binary")
	flag.StringVar(&cfg.Region, "region", "auto",
		"capture region WxH+X+Y or 'auto' for full display")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "show cimbar_recv output")
	flag.BoolVar(&cfg.KeepFrames, "keep", false,
		"keep captured frame images after decoding")
	flag.StringVar(&cfg.LogFile, "log", "",
		"write log to this file in addition to stderr (empty = console only)")
	flag.StringVar(&cfg.LogLevel, "loglevel", "info",
		"log verbosity: debug | info | warn | error")
	flag.Parse()
	return cfg
}

// defaultOutputDir returns %USERPROFILE%\Downloads on Windows, "." elsewhere.
func defaultOutputDir() string {
	if runtime.GOOS == "windows" {
		if home, err := os.UserHomeDir(); err == nil {
			dl := filepath.Join(home, "Downloads")
			if _, err := os.Stat(dl); err == nil {
				return dl
			}
		}
	}
	return "."
}

// ─── Receiver ────────────────────────────────────────────────────────────────

// Receiver orchestrates screen capture → detection → decoding.
type Receiver struct {
	cfg    Config
	region image.Rectangle
	log    *Logger
}

// Run is the main loop.
func (r *Receiver) Run() error {
	// ── Temp directory for frame images ───────────────────────────────────────
	framesDir, err := os.MkdirTemp("", "cimbar_frames_*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	if !r.cfg.KeepFrames {
		defer func() {
			r.log.Debug("removing temp frames dir: %s", framesDir)
			os.RemoveAll(framesDir)
		}()
	} else {
		r.log.Info("frame directory: %s", framesDir)
	}

	// ── Adaptive FPS controller ───────────────────────────────────────────────
	// minActiveFPS: never drop below 5fps once a code is detected.
	const minActiveFPS = 5.0
	adaptive := NewAdaptiveCapture(r.cfg.MinFPS, minActiveFPS, r.cfg.MaxFPS, r.log)

	// ── State ─────────────────────────────────────────────────────────────────
	var (
		uniqueFrameIdx  = 0       // saved (unique) frames
		totalCaptures   = 0       // all capture attempts
		detectedCimbar  = false
		firstDetectTime time.Time
		lastDetectTime  time.Time
		decodeStarted   = false
		decodeStartAt   time.Time
		resultCh        = make(chan DecoderResult, 1)
	)
	detectionDeadline := time.Now().Add(r.cfg.DetectTimeout)

	r.log.Info("scanning screen for cimbar code (timeout in %s)...", r.cfg.DetectTimeout)

	ticker := time.NewTicker(adaptive.Interval())
	defer ticker.Stop()

	for {
		select {
		// ── Periodic capture tick ─────────────────────────────────────────────
		case <-ticker.C:
			totalCaptures++

			// Capture screen.
			img, err := captureRegion(r.region)
			if err != nil {
				r.log.Warn("capture #%d failed: %v", totalCaptures, err)
				continue
			}

			// Let the adaptive controller observe the frame.
			// isNew == true means the animation advanced to a new frame.
			isNew := adaptive.Observe(img)
			r.log.Debug("capture #%d: isNew=%v fps=%.1f",
				totalCaptures, isNew, adaptive.CurrentFPS())

			// Update ticker interval if FPS changed significantly (>5%).
			newInterval := adaptive.Interval()
			ticker.Reset(newInterval)

			// Detect cimbar presence.
			present := hasCimbarPresence(img)

			if present {
				// ── Cimbar visible ─────────────────────────────────────────────
				if !detectedCimbar {
					detectedCimbar = true
					firstDetectTime = time.Now()
					adaptive.SetActive(true)
					r.log.Info("[OK] cimbar code detected on screen")
				}
				lastDetectTime = time.Now()

				// Save only unique frames to avoid feeding duplicates to cimbar_recv.
				if isNew {
					uniqueFrameIdx++
					framePath := filepath.Join(framesDir,
						fmt.Sprintf("frame_%05d.png", uniqueFrameIdx))
					if err := saveFrame(img, framePath); err != nil {
						r.log.Warn("save frame %d failed: %v", uniqueFrameIdx, err)
						continue
					}
					r.log.Debug("saved unique frame %d → %s", uniqueFrameIdx, framePath)
				}

				// Progress line (every second, approximately).
				if totalCaptures%max2(int(adaptive.CurrentFPS()), 1) == 0 {
					elapsed := time.Since(firstDetectTime).Round(time.Second)
					fmt.Printf("\r  unique frames: %d  total captures: %d  FPS: %.1f  elapsed: %s    ",
						uniqueFrameIdx, totalCaptures, adaptive.CurrentFPS(), elapsed)
				}

				// Start decoding once we have 5 seconds worth of unique frames.
				batchSize := int(minActiveFPS * 5) // ~25 unique frames minimum
				if !decodeStarted && uniqueFrameIdx >= batchSize {
					decodeStarted = true
					decodeStartAt = time.Now()
					pattern := filepath.Join(framesDir, "frame_%05d.png")
					r.log.Info("starting decode attempt (unique frames: %d, pattern: %s)",
						uniqueFrameIdx, pattern)
					fmt.Printf("\n\n")
					go runDecoder(r.cfg.CimbarBin, pattern, r.cfg.OutputDir,
						r.cfg.Verbose, resultCh)
				}

			} else {
				// ── Cimbar not visible ─────────────────────────────────────────
				if !detectedCimbar {
					// Still in the pre-detection wait window.
					remaining := time.Until(detectionDeadline)
					if remaining <= 0 {
						return fmt.Errorf(
							"no cimbar code detected within %s — "+
								"open https://cimbar.org in a browser and start the animation",
							r.cfg.DetectTimeout)
					}
					r.log.Debug("waiting for cimbar (%.0fs remaining)", remaining.Seconds())
					fmt.Printf("\r  Waiting for cimbar code... (%s remaining)    ",
						remaining.Round(time.Second))
				} else {
					// Code was visible but has now disappeared.
					gone := time.Since(lastDetectTime)
					if gone > 2*time.Second {
						adaptive.SetActive(false)
					}
					if gone > 3*time.Second && !decodeStarted {
						// Code gone for >3 s: browser animation probably finished.
						decodeStarted = true
						decodeStartAt = time.Now()
						pattern := filepath.Join(framesDir, "frame_%05d.png")
						r.log.Info("code disappeared (%.1fs) — final decode (%d unique frames)",
							gone.Seconds(), uniqueFrameIdx)
						fmt.Printf("\n\n")
						go runDecoder(r.cfg.CimbarBin, pattern, r.cfg.OutputDir,
							r.cfg.Verbose, resultCh)
					}
				}
			}

		// ── Decoder result ────────────────────────────────────────────────────
		case result := <-resultCh:
			if result.Err != nil {
				elapsed := time.Since(decodeStartAt)
				r.log.Warn("decode failed after %s: %v", elapsed.Round(time.Second), result.Err)

				if elapsed < 60*time.Second {
					// Keep capturing and try again later.
					decodeStarted = false
					r.log.Info("will retry decode after capturing more frames")
					continue
				}
				return fmt.Errorf("decode failed after %s: %w",
					elapsed.Round(time.Second), result.Err)
			}

			// ── Success ────────────────────────────────────────────────────────
			outPath := result.OutputFile
			if outPath == "" {
				outPath = newestFile(r.cfg.OutputDir, decodeStartAt.Add(-time.Second))
			}
			if outPath == "" {
				outPath = r.cfg.OutputDir + " (check directory for output file)"
			}

			r.log.Info("[OK] file received: %s", outPath)
			r.log.Info("stats: %d unique frames saved, %d total captures, %.1f fps at finish",
				uniqueFrameIdx, totalCaptures, adaptive.CurrentFPS())

			fmt.Printf("\n\n")
			notifySuccess("cimbar: file received!", "Saved to: "+outPath)
			termBell()
			return nil
		}
	}
}
