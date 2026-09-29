//go:build windows && amd64

package gdiplus

import "image"

// blurPasses of a box blur approximate a gaussian blur.
const blurPasses = 2

// clearRegion makes a region of a premultiplied BGRA buffer transparent.
func clearRegion(pixels []byte, width int, region image.Rectangle) {
	for y := region.Min.Y; y < region.Max.Y; y++ {
		row := (y*width + region.Min.X) * 4
		clear(pixels[row : row+region.Dx()*4])
	}
}

// blurRegion box-blurs a region in place; pixels outside the region count as
// transparent, so the shadow fades out at its edges.
func blurRegion(pixels []byte, width int, region image.Rectangle, radius int) {
	if radius <= 0 || region.Empty() {
		return
	}
	w, h := region.Dx(), region.Dy()
	out := make([]byte, max(w, h)*4)
	for range blurPasses {
		for y := region.Min.Y; y < region.Max.Y; y++ {
			base := (y*width + region.Min.X) * 4
			boxLine(pixels, base, 4, w, radius, out)
		}
		for x := region.Min.X; x < region.Max.X; x++ {
			base := (region.Min.Y*width + x) * 4
			boxLine(pixels, base, width*4, h, radius, out)
		}
	}
}

// boxLine blurs n pixels starting at base, stepping by step bytes; out is
// scratch space of at least 4n bytes.
func boxLine(pixels []byte, base, step, n, radius int, out []byte) {
	window := 2*radius + 1
	for c := 0; c < 4; c++ {
		sum := 0
		for i := 0; i <= radius && i < n; i++ {
			sum += int(pixels[base+i*step+c])
		}
		for i := 0; i < n; i++ {
			out[i*4+c] = byte(sum / window)
			if in := i + radius + 1; in < n {
				sum += int(pixels[base+in*step+c])
			}
			if gone := i - radius; gone >= 0 {
				sum -= int(pixels[base+gone*step+c])
			}
		}
	}
	for i := 0; i < n; i++ {
		copy(pixels[base+i*step:base+i*step+4], out[i*4:i*4+4])
	}
}

// compositeRegion lays src over dst ("source over", premultiplied alpha),
// with src scaled by opacity (0 to 255).
func compositeRegion(dst, src []byte, width int, region image.Rectangle, opacity int) {
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			i := (y*width + x) * 4
			alpha := (int(src[i+3])*opacity + 127) / 255
			if alpha == 0 {
				continue
			}
			inverse := 255 - alpha
			for c := range 4 {
				source := (int(src[i+c])*opacity + 127) / 255
				dst[i+c] = byte(source + (int(dst[i+c])*inverse+127)/255)
			}
		}
	}
}
