// image_png verifies that image.*, image/color.*, image/png, and image/jpeg
// are correctly intercepted.
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

func main() {
	// image: NewRGBA and basic methods.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_ = img.Bounds()

	// image: NewGray.
	gray := image.NewGray(image.Rect(0, 0, 4, 4))
	_ = gray

	// image: Pt constructor.
	pt := image.Pt(3, 5)
	_ = pt

	// image/color: RGBA model.
	c := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	r, g, b, a := c.RGBA()
	_, _, _, _ = r, g, b, a

	// image/png: Encode into a buffer, then Decode.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// intercept returns nil — should not reach here.
	}

	decoded, err := png.Decode(&buf)
	_ = decoded
	_ = err

	// image/jpeg: Encode and Decode.
	var jpegBuf bytes.Buffer
	_ = jpeg.Encode(&jpegBuf, img, nil)

	jpegDecoded, err2 := jpeg.Decode(&jpegBuf)
	_ = jpegDecoded
	_ = err2
}
