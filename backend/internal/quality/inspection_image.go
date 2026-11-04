package quality

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
)

// Locally generated, unambiguous visual evidence. Never fetch user-provided
// images or execute model output. The reference count is not sent upstream.
func inspectionImage() (string, string, error) {
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", "", err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 320, 320))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	count := 0
	for i, b := range seed {
		red := b%2 == 0
		if i == 0 {
			red = true
		}
		if i == 15 {
			red = false
		}
		c := color.RGBA{35, 100, 225, 255}
		if red {
			count++
			c = color.RGBA{225, 35, 35, 255}
		}
		x, y := 16+(i%4)*76, 16+(i/4)*76
		draw.Draw(canvas, image.Rect(x, y, x+60, y+60), image.NewUniform(c), image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return "", "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), strconv.Itoa(count), nil
}
