package captcha

import "testing"

func TestGenerateCreatesLegibleGlyphsForEverySupportedCharacter(t *testing.T) {
	for _, character := range alphabet {
		glyph, ok := glyphs[character]
		if !ok || len(glyph) != 7 {
			t.Fatalf("missing glyph for %q", character)
		}
	}
}
