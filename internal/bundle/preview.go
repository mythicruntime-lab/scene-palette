package bundle

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"scene-palette/internal/palette"
)

// makePreview uses a small built-in bitmap alphabet so headless bundles need
// neither platform fonts nor a browser. Browser callers may supply their own
// canvas PNG for exactly the same typography as the live app.
func makePreview(document palette.Document) ([]byte, error) {
	tk, err := tokens(document)
	if err != nil {
		return nil, err
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 960, 250))
	paint := func(index int) color.NRGBA {
		rgb := tk[index].RGB
		return color.NRGBA{R: uint8(rgb.R), G: uint8(rgb.G), B: uint8(rgb.B), A: 255}
	}
	ground, ink := paint(0), paint(2)
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(ground), image.Point{}, draw.Src)
	drawText(canvas, 18, 14, 3, document.Name, ink, 51)
	drawText(canvas, 18, 44, 1, document.Polarity+" / SCENE PALETTE", ink, 140)
	for index, token := range tk[:8] {
		x := index * 120
		draw.Draw(canvas, image.Rect(x, 62, x+118, 166), image.NewUniform(paint(index)), image.Point{}, draw.Src)
		drawText(canvas, x+4, 174, 2, token.Hex, ink, 9)
		drawText(canvas, x+4, 194, 1, token.Key, ink, 18)
	}
	if len(tk) > 8 {
		width := 960 / (len(tk) - 8)
		if width > 120 {
			width = 120
		}
		for index := 8; index < len(tk); index++ {
			x := (index - 8) * width
			draw.Draw(canvas, image.Rect(x, 210, x+max(1, width-2), 240), image.NewUniform(paint(index)), image.Point{}, draw.Src)
		}
	}
	var result bytes.Buffer
	err = png.Encode(&result, canvas)
	return result.Bytes(), err
}

func drawText(dst *image.NRGBA, x, y, scale int, value string, ink color.NRGBA, limit int) {
	for index, letter := range []rune(strings.ToUpper(value)) {
		if index >= limit {
			break
		}
		glyph, ok := glyphs[letter]
		if !ok {
			glyph = glyphs['?']
		}
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<(4-col)) != 0 {
					rect := image.Rect(x+(index*6+col)*scale, y+row*scale, x+(index*6+col+1)*scale, y+(row+1)*scale)
					draw.Draw(dst, rect, image.NewUniform(ink), image.Point{}, draw.Src)
				}
			}
		}
	}
}

var glyphs = map[rune][7]byte{
	' ': {}, '#': {10, 31, 10, 10, 31, 10, 0}, '-': {0, 0, 0, 31, 0, 0, 0}, '_': {0, 0, 0, 0, 0, 0, 31},
	'.': {0, 0, 0, 0, 0, 12, 12}, ':': {0, 12, 12, 0, 12, 12, 0}, '/': {1, 2, 2, 4, 8, 8, 16}, '?': {14, 17, 1, 2, 4, 0, 4},
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14}, '2': {14, 17, 1, 2, 4, 8, 31},
	'3': {30, 1, 1, 14, 1, 1, 30}, '4': {2, 6, 10, 18, 31, 2, 2}, '5': {31, 16, 16, 30, 1, 1, 30},
	'6': {14, 16, 16, 30, 17, 17, 14}, '7': {31, 1, 2, 4, 8, 8, 8}, '8': {14, 17, 17, 14, 17, 17, 14}, '9': {14, 17, 17, 15, 1, 1, 14},
	'A': {14, 17, 17, 31, 17, 17, 17}, 'B': {30, 17, 17, 30, 17, 17, 30}, 'C': {14, 17, 16, 16, 16, 17, 14},
	'D': {30, 17, 17, 17, 17, 17, 30}, 'E': {31, 16, 16, 30, 16, 16, 31}, 'F': {31, 16, 16, 30, 16, 16, 16},
	'G': {14, 17, 16, 23, 17, 17, 15}, 'H': {17, 17, 17, 31, 17, 17, 17}, 'I': {14, 4, 4, 4, 4, 4, 14},
	'J': {7, 2, 2, 2, 2, 18, 12}, 'K': {17, 18, 20, 24, 20, 18, 17}, 'L': {16, 16, 16, 16, 16, 16, 31},
	'M': {17, 27, 21, 21, 17, 17, 17}, 'N': {17, 25, 21, 19, 17, 17, 17}, 'O': {14, 17, 17, 17, 17, 17, 14},
	'P': {30, 17, 17, 30, 16, 16, 16}, 'Q': {14, 17, 17, 17, 21, 18, 13}, 'R': {30, 17, 17, 30, 20, 18, 17},
	'S': {15, 16, 16, 14, 1, 1, 30}, 'T': {31, 4, 4, 4, 4, 4, 4}, 'U': {17, 17, 17, 17, 17, 17, 14},
	'V': {17, 17, 17, 17, 17, 10, 4}, 'W': {17, 17, 17, 21, 21, 21, 10}, 'X': {17, 17, 10, 4, 10, 17, 17},
	'Y': {17, 17, 10, 4, 4, 4, 4}, 'Z': {31, 1, 2, 4, 8, 16, 31},
}
