package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/kbinani/screenshot"
)

// DisplayInfo holds info about a chosen display.
type DisplayInfo struct {
	Index  int
	Bounds image.Rectangle
}

func getDisplay(idx int) (DisplayInfo, error) {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return DisplayInfo{}, fmt.Errorf("no active displays found")
	}
	if idx < 0 || idx >= n {
		return DisplayInfo{}, fmt.Errorf("display %d not found (%d display(s) available)", idx, n)
	}
	return DisplayInfo{
		Index:  idx,
		Bounds: screenshot.GetDisplayBounds(idx),
	}, nil
}

// parseRegion parses a region string "WxH+X+Y" or returns the full display bounds.
func parseRegion(spec string, display DisplayInfo) (image.Rectangle, error) {
	if spec == "" || spec == "auto" {
		return display.Bounds, nil
	}
	// Format: WxH+X+Y
	var w, h, x, y int
	n, err := fmt.Sscanf(strings.ReplaceAll(spec, "+", " "), "%dx%d %d %d", &w, &h, &x, &y)
	if err != nil || n != 4 {
		return image.Rectangle{}, fmt.Errorf("invalid region %q: expected WxH+X+Y", spec)
	}
	return image.Rect(x, y, x+w, y+h), nil
}

// captureRegion captures a rectangle from the display using the platform GDI/API.
// On Windows this uses BitBlt via github.com/kbinani/screenshot.
func captureRegion(region image.Rectangle) (image.Image, error) {
	img, err := screenshot.CaptureRect(region)
	if err != nil {
		return nil, fmt.Errorf("screen capture failed: %w", err)
	}
	return img, nil
}

// saveFrame writes an image to path as PNG.
func saveFrame(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
