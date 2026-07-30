package labelmaker

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

// ZPL generation mirrors dylanl321/zebra-label-maker app.js (generateZPL / getLabelDimensions).
const zebraDPI = 203

var zebraLabelPresets = map[string]struct{ W, H float64 }{
	"1x1":       {1, 1},
	"2.25x1.25": {2.25, 1.25},
	"4x6":       {4, 6},
	"2x1":       {2, 1},
	"3x2":       {3, 2},
	"4x2":       {4, 2},
}

type zplLine struct {
	Type  string // text | qr | separator | barcode
	Text  string
	Data  string
	Style string // header | bold | center | right | normal
}

type zplDims struct {
	WDots int
	HDots int
}

// GenerateZPL builds ZPL II the same way zebra-label-maker does: vertically stacked,
// horizontally centered content on a preset label size (default 2x1 @ 203 DPI).
func GenerateZPL(params *GenerateParameters, cfg *config.Config) string {
	dim := zebraDimensions(cfg)
	fontSize := 30
	darkness := 15
	speed := 4
	if cfg != nil {
		if cfg.LabelMaker.PrintFontSize > 0 {
			fontSize = cfg.LabelMaker.PrintFontSize
		}
		if cfg.LabelMaker.PrintDarkness > 0 {
			darkness = cfg.LabelMaker.PrintDarkness
		}
		if cfg.LabelMaker.PrintSpeed > 0 {
			speed = cfg.LabelMaker.PrintSpeed
		}
	}
	headerSize := int(math.Round(float64(fontSize) * 1.8))
	lineHeight := fontSize + 12
	headerLineHeight := headerSize + 16

	lines := homeboxLabelLines(params)

	totalHeight := 0
	for _, line := range lines {
		switch line.Type {
		case "separator":
			totalHeight += 16
		case "barcode":
			totalHeight += 90
		case "qr":
			totalHeight += 120
		default:
			if line.Style == "header" {
				totalHeight += headerLineHeight
			} else {
				totalHeight += lineHeight
			}
		}
	}

	y := int(math.Max(10, math.Round(float64(dim.HDots-totalHeight)/2)))

	var b strings.Builder
	b.WriteString("^XA\n")
	fmt.Fprintf(&b, "^PW%d\n", dim.WDots)
	fmt.Fprintf(&b, "^LL%d\n", dim.HDots)
	fmt.Fprintf(&b, "~SD%d\n", darkness)
	fmt.Fprintf(&b, "^PR%d\n", speed)
	fmt.Fprintf(&b, "^CF0,%d\n", fontSize)

	for _, line := range lines {
		switch line.Type {
		case "separator":
			fmt.Fprintf(&b, "^FO10,%d^GB%d,1,2^FS\n", y, dim.WDots-20)
			y += 16
		case "barcode":
			barcodeWidth := len(line.Data) * 2 * 11
			bx := int(math.Max(20, math.Round(float64(dim.WDots-barcodeWidth)/2)))
			fmt.Fprintf(&b, "^FO%d,%d^BY2,2,60^BCN,60,Y,N,N^FD%s^FS\n", bx, y, escapeZPLField(line.Data))
			y += 90
		case "qr":
			qx := int(math.Round(float64(dim.WDots-100) / 2))
			fmt.Fprintf(&b, "^FO%d,%d^BQN,2,5^FDQA,%s^FS\n", qx, y, escapeZPLField(line.Data))
			y += 120
		default:
			fSize, fWidth := fontSize, fontSize
			if line.Style == "header" {
				fSize, fWidth = headerSize, headerSize
			} else if line.Style == "bold" {
				fWidth = fontSize + 8
			}
			if line.Style == "right" {
				fmt.Fprintf(&b, "^FO0,%d^A0N,%d,%d^FB%d,1,0,R^FD%s^FS\n", y, fSize, fWidth, dim.WDots-20, escapeZPLField(line.Text))
			} else {
				fmt.Fprintf(&b, "^FO0,%d^A0N,%d,%d^FB%d,1,0,C^FD%s^FS\n", y, fSize, fWidth, dim.WDots, escapeZPLField(line.Text))
			}
			if line.Style == "header" {
				y += headerLineHeight
			} else {
				y += lineHeight
			}
		}
	}

	b.WriteString("^XZ")
	return b.String()
}

func zebraDimensions(cfg *config.Config) zplDims {
	size := "2x1"
	orientation := "portrait"
	if cfg != nil {
		if s := strings.TrimSpace(cfg.LabelMaker.LabelSize); s != "" {
			size = s
		}
		if o := strings.TrimSpace(strings.ToLower(cfg.LabelMaker.Orientation)); o != "" {
			orientation = o
		}
	}
	preset, ok := zebraLabelPresets[size]
	if !ok {
		preset = zebraLabelPresets["2x1"]
	}
	w, h := preset.W, preset.H
	if orientation == "landscape" {
		w, h = h, w
	}
	return zplDims{
		WDots: int(math.Round(w * zebraDPI)),
		HDots: int(math.Round(h * zebraDPI)),
	}
}

// homeboxLabelLines maps Homebox label fields into zebra-label-maker line types:
//
//	#Title
//	description lines…
//	[qr:url]
//	optional additional info
func homeboxLabelLines(params *GenerateParameters) []zplLine {
	var lines []zplLine
	if title := strings.TrimSpace(params.TitleText); title != "" {
		lines = append(lines, zplLine{Type: "text", Text: title, Style: "header"})
	}
	for _, part := range strings.Split(params.DescriptionText, "\n") {
		if text := strings.TrimSpace(part); text != "" {
			lines = append(lines, zplLine{Type: "text", Text: text, Style: "normal"})
		}
	}
	if url := strings.TrimSpace(params.URL); url != "" {
		lines = append(lines, zplLine{Type: "qr", Data: url})
	}
	if params.AdditionalInformation != nil {
		if text := strings.TrimSpace(*params.AdditionalInformation); text != "" {
			lines = append(lines, zplLine{Type: "text", Text: text, Style: "normal"})
		}
	}
	return lines
}

// escapeZPLField strips ZPL control characters from field data.
func escapeZPLField(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '^', '~', '\\':
			return ' '
		default:
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}
	}, s)
	return s
}
