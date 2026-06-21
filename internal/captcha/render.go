package captcha

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Image dimensions of the rendered challenge.
const (
	imgW = 170
	imgH = 60
)

// renderPNG draws code into a distorted PNG: a light background, per-glyph
// scaling and vertical jitter, speckle noise, and a few random lines.
func renderPNG(code string) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	// Light, slightly varied background.
	bg := color.RGBA{
		R: 225 + uint8(rand.IntN(25)),
		G: 225 + uint8(rand.IntN(25)),
		B: 225 + uint8(rand.IntN(25)),
		A: 255,
	}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// A couple of faint lines behind the text.
	for i := 0; i < 3; i++ {
		drawLine(dst,
			rand.IntN(imgW), rand.IntN(imgH),
			rand.IntN(imgW), rand.IntN(imgH),
			color.RGBA{uint8(rand.IntN(160)), uint8(rand.IntN(160)), uint8(rand.IntN(160)), 255})
	}

	// Draw each character into its own small image, then scale it up onto the
	// canvas with random scale, position jitter, and color.
	cellW := imgW / (len(code) + 1)
	for i, ch := range code {
		glyph := renderGlyph(byte(ch))
		scale := 2.8 + rand.Float64()*0.9 // 2.8x .. 3.7x
		gw := int(float64(glyph.Bounds().Dx()) * scale)
		gh := int(float64(glyph.Bounds().Dy()) * scale)
		x := cellW*i + cellW/2 + rand.IntN(7) - 3
		y := (imgH-gh)/2 + rand.IntN(14) - 7
		draw.CatmullRom.Scale(dst, image.Rect(x, y, x+gw, y+gh), glyph, glyph.Bounds(), draw.Over, nil)
	}

	// Speckle noise on top.
	for i := 0; i < 450; i++ {
		dst.Set(rand.IntN(imgW), rand.IntN(imgH),
			color.RGBA{uint8(rand.IntN(256)), uint8(rand.IntN(256)), uint8(rand.IntN(256)), 255})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderGlyph draws a single character in a dark random color onto a small
// transparent image sized to the base font cell.
func renderGlyph(ch byte) *image.RGBA {
	face := basicfont.Face7x13
	img := image.NewRGBA(image.Rect(0, 0, face.Advance, face.Height))
	col := color.RGBA{uint8(rand.IntN(90)), uint8(rand.IntN(90)), uint8(rand.IntN(90)), 255}
	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(0, face.Ascent),
	}
	d.DrawString(string(ch))
	return img
}

// drawLine plots a 2px-thick line between two points (Bresenham).
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		img.Set(x0, y0, c)
		img.Set(x0, y0+1, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
