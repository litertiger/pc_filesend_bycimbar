// cimbar-recv-pc — PC-side receiver for cimbar encoded data
//
// Usage:
//
//	cimbar-recv-pc [flags]
//
// The program captures the local screen, detects when a cimbar code is being
// displayed (e.g. from https://cimbar.org in a browser), decodes it using the
// external cimbar_recv binary, and saves the resulting file.
//
// Flags:
//
//	-output    Output directory (default: current directory)
//	-timeout   How long to wait before giving up if no code appears (default: 20s)
//	-fps       Screen capture frames per second (default: 5)
//	-display   Display index (default: 0)
//	-cimbar    Path to cimbar_recv binary (default: cimbar_recv from PATH)
//	-region    Capture region WxH+X+Y, or "auto" for full display (default: auto)
//	-verbose   Print cimbar_recv output (default: false)
//	-keep      Keep captured frames after decoding (default: false)
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

	fmt.Println("=== cimbar PC Receiver ===")
	fmt.Printf("Output directory : %s\n", cfg.OutputDir)
	fmt.Printf("Detection timeout: %s\n", cfg.DetectTimeout)
	fmt.Printf("Capture FPS      : %d\n", cfg.FPS)
	fmt.Printf("Display          : %d\n", cfg.Display)
	fmt.Printf("cimbar_recv      : %s\n\n", cfg.CimbarBin)

	if err := checkCimbarBin(cfg.CimbarBin); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	display, err := getDisplay(cfg.Display)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	region, err := parseRegion(cfg.Region, display)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("Capture region   : %dx%d at (%d,%d)\n\n",
		region.Dx(), region.Dy(), region.Min.X, region.Min.Y)

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR creating output dir:", err)
		os.Exit(1)
	}

	r := &Receiver{cfg: cfg, region: region}
	if err := r.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "\nERROR:", err)
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
	FPS           int
	Display       int
	CimbarBin     string
	Region        string
	Verbose       bool
	KeepFrames    bool
}

func parseFlags() Config {
	var cfg Config
	flag.StringVar(&cfg.OutputDir, "output", defaultOutputDir(), "output directory for received files")
	flag.DurationVar(&cfg.DetectTimeout, "timeout", 20*time.Second, "timeout if no cimbar code detected on screen")
	flag.IntVar(&cfg.FPS, "fps", 5, "screen capture frames per second")
	flag.IntVar(&cfg.Display, "display", 0, "display index to monitor")
	flag.StringVar(&cfg.CimbarBin, "cimbar", defaultCimbarBin(), "path to cimbar_recv binary")
	flag.StringVar(&cfg.Region, "region", "auto", "capture region WxH+X+Y or 'auto' for full display")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "show cimbar_recv output")
	flag.BoolVar(&cfg.KeepFrames, "keep", false, "keep captured frame images after decoding")
	flag.Parse()
	return cfg
}

// defaultOutputDir returns the user's Downloads folder on Windows,
// falling back to the current directory on other platforms.
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

// defaultCimbarBin returns the expected cimbar_recv executable name.
// On Windows, exec.LookPath already appends .exe automatically, so we
// keep the base name for consistency across platforms.
func defaultCimbarBin() string {
	return "cimbar_recv"
}

// ─── Receiver ────────────────────────────────────────────────────────────────

// Receiver orchestrates screen capture → detection → decoding.
type Receiver struct {
	cfg    Config
	region image.Rectangle
}

// Run is the main loop.
func (r *Receiver) Run() error {
	// Create a temporary directory for captured frame images.
	framesDir, err := os.MkdirTemp("", "cimbar_frames_*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	if !r.cfg.KeepFrames {
		defer os.RemoveAll(framesDir)
	} else {
		fmt.Println("Frame directory:", framesDir)
	}

	region := r.region
	interval := time.Duration(float64(time.Second) / float64(r.cfg.FPS))

	fmt.Println("Scanning screen for cimbar code...")
	fmt.Printf("(will time out in %s if no code is detected)\n\n", r.cfg.DetectTimeout)

	var (
		frameIdx        = 0
		detectedCimbar  = false
		firstDetectTime time.Time
		lastDetectTime  time.Time
		decodeStarted   = false
		decodeStartAt   time.Time
		resultCh        = make(chan DecoderResult, 1)
	)

	// detectionDeadline: if no cimbar detected by this time → error.
	detectionDeadline := time.Now().Add(r.cfg.DetectTimeout)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// ── Capture frame ──────────────────────────────────────────────────
			img, err := captureRegion(region)
			if err != nil {
				fmt.Fprintf(os.Stderr, "capture error: %v\n", err)
				continue
			}

			// ── Detect cimbar presence ─────────────────────────────────────────
			present := hasCimbarPresence(img)

			if present {
				if !detectedCimbar {
					detectedCimbar = true
					firstDetectTime = time.Now()
					fmt.Println("[OK] Cimbar code detected! Capturing frames...")
				}
				lastDetectTime = time.Now()

				// Save the frame.
				frameIdx++
				framePath := filepath.Join(framesDir, fmt.Sprintf("frame_%05d.png", frameIdx))
				if err := saveFrame(img, framePath); err != nil {
					fmt.Fprintf(os.Stderr, "save frame error: %v\n", err)
					continue
				}

				// Print progress indicator.
				if frameIdx%r.cfg.FPS == 0 {
					elapsed := time.Since(firstDetectTime).Round(time.Second)
					fmt.Printf("\r  Captured %d frames (%s)...    ", frameIdx, elapsed)
				}

				// After accumulating enough frames, start decoding.
				// We attempt a decode every 5 seconds of captured frames.
				batchSize := r.cfg.FPS * 5
				if !decodeStarted && frameIdx >= batchSize {
					decodeStarted = true
					decodeStartAt = time.Now()
					pattern := filepath.Join(framesDir, "frame_%05d.png")
					fmt.Printf("\n\nStarting decode (frame 1..%d)...\n", frameIdx)
					go runDecoder(r.cfg.CimbarBin, pattern, r.cfg.OutputDir, r.cfg.Verbose, resultCh)
				}
			} else {
				// No code visible right now.
				if !detectedCimbar {
					// Still waiting for first detection.
					remaining := time.Until(detectionDeadline)
					if remaining <= 0 {
						return fmt.Errorf("no cimbar code detected within %s — make sure the browser is showing a cimbar animation", r.cfg.DetectTimeout)
					}
					fmt.Printf("\r  Waiting... (%s remaining)    ", remaining.Round(time.Second))
				} else {
					// Code disappeared after being seen.
					gone := time.Since(lastDetectTime)
					if gone > 3*time.Second {
						// Code gone for >3s: assume transfer ended; re-run decode on everything.
						if !decodeStarted {
							decodeStarted = true
							decodeStartAt = time.Now()
							pattern := filepath.Join(framesDir, "frame_%05d.png")
							fmt.Printf("\n\nCode disappeared — starting final decode (%d frames)...\n", frameIdx)
							go runDecoder(r.cfg.CimbarBin, pattern, r.cfg.OutputDir, r.cfg.Verbose, resultCh)
						}
					}
				}
			}

		case result := <-resultCh:
			// ── Decoder finished ───────────────────────────────────────────────
			if result.Err != nil {
				if decodeStarted && frameIdx < r.cfg.FPS*5 {
					// Not enough frames yet; keep capturing.
					decodeStarted = false
					continue
				}
				// Enough frames but still failing; try to get more.
				elapsed := time.Since(decodeStartAt)
				if elapsed < 60*time.Second {
					fmt.Printf("\n  Decode attempt failed (%v), continuing capture...\n", result.Err)
					decodeStarted = false
					continue
				}
				return fmt.Errorf("decode failed after %s: %w", elapsed.Round(time.Second), result.Err)
			}

			// Success!
			outPath := result.OutputFile
			if outPath == "" {
				// Look for any newly created file in outputDir.
				outPath = newestFile(r.cfg.OutputDir, decodeStartAt.Add(-time.Second))
			}
			if outPath == "" {
				outPath = r.cfg.OutputDir + " (check directory for output file)"
			}

			fmt.Printf("\n\n")
			notifySuccess("cimbar: file received!", "Saved to: "+outPath)
			termBell()
			return nil
		}
	}
}
