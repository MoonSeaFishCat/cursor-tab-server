package captcha

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"time"
)

const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

var glyphs = map[rune][]string{
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'J': {"00111", "00010", "00010", "00010", "00010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
}

type Challenge struct {
	ID         string
	Answer     string
	AnswerHash []byte
	Image      []byte
	ExpiresAt  time.Time
}

func Generate(now time.Time) (Challenge, error) {
	answer, err := randomText(6)
	if err != nil {
		return Challenge{}, err
	}
	id, err := randomText(24)
	if err != nil {
		return Challenge{}, err
	}
	imageBytes, err := render(answer)
	if err != nil {
		return Challenge{}, err
	}
	hash := sha256.Sum256([]byte(answer))
	return Challenge{ID: id, Answer: answer, AnswerHash: hash[:], Image: imageBytes, ExpiresAt: now.UTC().Add(5 * time.Minute)}, nil
}

func randomText(length int) (string, error) {
	text := make([]byte, length)
	for index := range text {
		value, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		text[index] = alphabet[value.Int64()]
	}
	return string(text), nil
}

func render(answer string) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 180, 58))
	background := color.RGBA{245, 246, 248, 255}
	for y := 0; y < 58; y++ {
		for x := 0; x < 180; x++ {
			canvas.Set(x, y, background)
		}
	}
	for index := 0; index < 7; index++ {
		y := 8 + index*7
		for x := 0; x < 180; x++ {
			if (x+index*13)%19 == 0 {
				canvas.Set(x, y, color.RGBA{160, 170, 185, 255})
			}
		}
	}
	for index, character := range answer {
		drawGlyph(canvas, 12+index*27, 12+(index%2)*3, character)
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode captcha: %w", err)
	}
	return output.Bytes(), nil
}

func drawGlyph(canvas *image.RGBA, startX, startY int, character rune) {
	glyph := glyphs[character]
	for row, pixels := range glyph {
		for column, pixel := range pixels {
			if pixel != '1' {
				continue
			}
			for y := 0; y < 3; y++ {
				for x := 0; x < 3; x++ {
					canvas.Set(startX+column*3+x, startY+row*3+y, color.RGBA{31, 41, 55, 255})
				}
			}
		}
	}
}

func DataURL(pngBytes []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}
