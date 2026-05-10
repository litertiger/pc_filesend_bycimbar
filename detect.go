package main

import (
	"image"
	"image/color"
	"math"
)

// hasCimbarPresence returns true if the image likely contains a cimbar code.
//
// Detection strategy:
//  1. The three cimbar anchor squares sit in top-left, top-right and bottom-left
//     corners.  Each anchor is a nested dark→light→dark square pattern.
//  2. Inside the code area the data tiles use bright, highly-saturated colors.
//
// We check both conditions: dark corners + saturated interior.
func hasCimbarPresence(img image.Image) bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 100 || h < 100 {
		return false
	}

	// ── Step 1: look for nested dark/light/dark squares at the three expected corners ──
	// We scan for the largest dark square we can find in each corner quadrant
	// and verify the nesting pattern.
	cornerSize := min2(w, h) / 8 // rough anchor region width

	tlOk := checkAnchorCorner(img, b.Min.X, b.Min.Y, cornerSize, cornerSize)
	trOk := checkAnchorCorner(img, b.Max.X-cornerSize, b.Min.Y, cornerSize, cornerSize)
	blOk := checkAnchorCorner(img, b.Min.X, b.Max.Y-cornerSize, cornerSize, cornerSize)

	if !tlOk || !trOk || !blOk {
		return false
	}

	// ── Step 2: center region has brightly-colored (saturated) pixels ──
	cx0 := b.Min.X + w/4
	cy0 := b.Min.Y + h/4
	cx1 := b.Max.X - w/4
	cy1 := b.Max.Y - h/4
	return hasSaturatedRegion(img, cx0, cy0, cx1, cy1, 0.20)
}

// checkAnchorCorner returns true when a corner region (rx,ry,rw,rh) shows
// the nested dark→light→dark pattern expected of a cimbar anchor square.
func checkAnchorCorner(img image.Image, rx, ry, rw, rh int) bool {
	if rw < 10 || rh < 10 {
		return false
	}

	// Outer band (outermost ~20% of the short side): should be mostly dark.
	edge := min2(rw, rh) / 5
	if edge < 2 {
		edge = 2
	}
	outerDark := sampledDarkRatio(img, rx, ry, rx+rw, ry+rh, rx+edge, ry+edge, rx+rw-edge, ry+rh-edge)

	// Middle band: should be mostly light.
	mid := edge
	midDark := sampledDarkRatio(img, rx+edge, ry+edge, rx+rw-edge, ry+rh-edge,
		rx+edge+mid, ry+edge+mid, rx+rw-edge-mid, ry+rh-edge-mid)
	midLight := 1 - midDark

	// Inner square: should be mostly dark.
	ix0 := rx + edge + mid
	iy0 := ry + edge + mid
	ix1 := rx + rw - edge - mid
	iy1 := ry + rh - edge - mid
	if ix1 <= ix0 || iy1 <= iy0 {
		// Region too small to evaluate inner square; fall back to outer only.
		return outerDark > 0.55
	}
	innerDark := sampledDarkRatio(img, ix0, iy0, ix1, iy1, ix0, iy0, ix1, iy1)

	return outerDark > 0.50 && midLight > 0.50 && innerDark > 0.45
}

// sampledDarkRatio samples pixels in the ring defined by outer rect minus inner rect.
// Returns fraction of sampled pixels that are "dark" (luminance < 80).
func sampledDarkRatio(img image.Image, ox0, oy0, ox1, oy1, ix0, iy0, ix1, iy1 int) float64 {
	dark, total := 0, 0
	step := max2((ox1-ox0)/12, 1)
	for y := oy0; y < oy1; y += step {
		for x := ox0; x < ox1; x += step {
			if x >= ix0 && x < ix1 && y >= iy0 && y < iy1 {
				continue // inside the hole
			}
			if isDark(img.At(x, y)) {
				dark++
			}
			total++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(dark) / float64(total)
}

// hasSaturatedRegion returns true if at least minFraction of sampled pixels
// inside (x0,y0)-(x1,y1) have HSV saturation > 0.35 and value > 0.25.
func hasSaturatedRegion(img image.Image, x0, y0, x1, y1 int, minFraction float64) bool {
	if x1 <= x0 || y1 <= y0 {
		return false
	}
	sat, total := 0, 0
	step := max2((x1-x0)/20, 1)
	for y := y0; y < y1; y += step {
		for x := x0; x < x1; x += step {
			r, g, b := rgbFloat(img.At(x, y))
			if isSaturated(r, g, b) {
				sat++
			}
			total++
		}
	}
	if total == 0 {
		return false
	}
	return float64(sat)/float64(total) >= minFraction
}

func isDark(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	lum := (299*r + 587*g + 114*b) / 1000
	return lum < 0x2000 // < ~12% brightness (scale 0-65535)
}

func rgbFloat(c color.Color) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return float64(r) / 65535, float64(g) / 65535, float64(b) / 65535
}

// isSaturated returns true for pixels with HSV saturation > 0.35 and value > 0.20.
func isSaturated(r, g, b float64) bool {
	maxC := math.Max(r, math.Max(g, b))
	minC := math.Min(r, math.Min(g, b))
	if maxC < 0.20 {
		return false // too dark
	}
	sat := (maxC - minC) / maxC
	return sat > 0.35
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
