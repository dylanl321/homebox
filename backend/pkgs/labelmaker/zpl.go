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

type zplDims struct {
	WDots int
	HDots int
}

type zplTextLine struct {
	Text string
	Font int
	Y    int
	Bold bool
}

type zplLayout struct {
	Width     int
	Height    int
	Margin    int
	QRX       int
	QRY       int
	QRMag     int
	QRSize    int
	TextX     int
	TextWidth int
	TextLines []zplTextLine
}

// GenerateZPL creates a bounded side-by-side layout: QR on the left and text on
// the right. Text is wrapped, font-reduced, line-limited, and truncated before
// ZPL is emitted so fields cannot overlap each other or the QR region.
func GenerateZPL(params *GenerateParameters, cfg *config.Config) string {
	layout := buildZPLLayout(params, cfg)
	darkness := 15
	speed := 4
	if cfg != nil {
		if cfg.LabelMaker.PrintDarkness > 0 {
			darkness = cfg.LabelMaker.PrintDarkness
		}
		if cfg.LabelMaker.PrintSpeed > 0 {
			speed = cfg.LabelMaker.PrintSpeed
		}
	}
	var b strings.Builder
	b.WriteString("^XA\n")
	fmt.Fprintf(&b, "^PW%d\n", layout.Width)
	fmt.Fprintf(&b, "^LL%d\n", layout.Height)
	fmt.Fprintf(&b, "~SD%d\n", darkness)
	fmt.Fprintf(&b, "^PR%d\n", speed)

	if url := strings.TrimSpace(params.URL); url != "" {
		fmt.Fprintf(
			&b,
			"^FO%d,%d^BQN,2,%d^FDQA,%s^FS\n",
			layout.QRX,
			layout.QRY,
			layout.QRMag,
			escapeZPLField(url),
		)
	}

	for _, line := range layout.TextLines {
		fontWidth := line.Font
		if line.Bold {
			fontWidth += 2
		}
		fmt.Fprintf(
			&b,
			"^FO%d,%d^A0N,%d,%d^FB%d,1,0,L^FD%s^FS\n",
			layout.TextX,
			line.Y,
			line.Font,
			fontWidth,
			layout.TextWidth,
			escapeZPLField(line.Text),
		)
	}

	b.WriteString("^XZ")
	return b.String()
}

func buildZPLLayout(params *GenerateParameters, cfg *config.Config) zplLayout {
	dim := zebraDimensions(cfg)
	minDimension := min(dim.WDots, dim.HDots)
	margin := clamp(minDimension/16, 8, 20)
	gap := clamp(minDimension/20, 8, 16)

	layout := zplLayout{
		Width:  dim.WDots,
		Height: dim.HDots,
		Margin: margin,
	}

	availableHeight := dim.HDots - (2 * margin)
	hasQR := strings.TrimSpace(params.URL) != ""
	if hasQR {
		maxQRRegion := int(math.Round(float64(dim.WDots) * 0.44))
		qrRegion := min(availableHeight, maxQRRegion)
		qrRegion = max(qrRegion, minDimension/2)
		qrRegion = min(qrRegion, dim.WDots-(2*margin)-gap-48)

		modules := estimatedQRModules(params.URL)
		layout.QRMag = clamp(qrRegion/modules, 1, 8)
		layout.QRSize = modules * layout.QRMag
		layout.QRX = margin + max(0, (qrRegion-layout.QRSize)/2)
		layout.QRY = margin + max(0, (availableHeight-layout.QRSize)/2)
		layout.TextX = margin + qrRegion + gap
	} else {
		layout.TextX = margin
	}
	layout.TextWidth = max(32, dim.WDots-layout.TextX-margin)

	titleFont := 30
	if cfg != nil && cfg.LabelMaker.PrintFontSize > 0 {
		titleFont = cfg.LabelMaker.PrintFontSize
	}
	titleFont = clamp(titleFont, 14, max(14, availableHeight/3))

	title := normalizeLabelText(params.TitleText)
	var titleLines []string
	for font := titleFont; font >= 14; font -= 2 {
		candidate, truncated := wrapAndTruncate(title, charsForWidth(layout.TextWidth, font), 2)
		titleFont = font
		titleLines = candidate
		if !truncated || font == 14 {
			break
		}
	}

	description := normalizeLabelText(params.DescriptionText)
	if params.AdditionalInformation != nil {
		additional := normalizeLabelText(*params.AdditionalInformation)
		if additional != "" {
			if description != "" {
				description += " | "
			}
			description += additional
		}
	}

	titleLineHeight := titleFont + 4
	titleHeight := len(titleLines) * titleLineHeight
	bodyFont := clamp(int(math.Round(float64(titleFont)*0.72)), 12, 30)
	bodyLineHeight := bodyFont + 4
	bodyLinesAvailable := max(0, (availableHeight-titleHeight-4)/bodyLineHeight)
	bodyLines := wrapTextBounded(description, charsForWidth(layout.TextWidth, bodyFont), bodyLinesAvailable)

	totalTextHeight := titleHeight
	if len(bodyLines) > 0 {
		totalTextHeight += 4 + (len(bodyLines) * bodyLineHeight)
	}
	y := margin + max(0, (availableHeight-totalTextHeight)/2)
	for _, text := range titleLines {
		layout.TextLines = append(layout.TextLines, zplTextLine{Text: text, Font: titleFont, Y: y, Bold: true})
		y += titleLineHeight
	}
	if len(bodyLines) > 0 {
		y += 4
	}
	for _, text := range bodyLines {
		layout.TextLines = append(layout.TextLines, zplTextLine{Text: text, Font: bodyFont, Y: y})
		y += bodyLineHeight
	}

	return layout
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

func estimatedQRModules(data string) int {
	switch length := len([]byte(data)); {
	case length <= 20:
		return 25
	case length <= 35:
		return 29
	case length <= 50:
		return 33
	case length <= 70:
		return 37
	case length <= 90:
		return 41
	case length <= 120:
		return 45
	default:
		return 49
	}
}

func charsForWidth(width, font int) int {
	if font <= 0 {
		return 1
	}
	return max(1, int(math.Floor(float64(width)/(float64(font)*0.58))))
}

func normalizeLabelText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func wrapTextBounded(text string, maxChars, maxLines int) []string {
	lines, _ := wrapAndTruncate(text, maxChars, maxLines)
	return lines
}

func wrapAndTruncate(text string, maxChars, maxLines int) ([]string, bool) {
	text = normalizeLabelText(text)
	if text == "" || maxChars <= 0 || maxLines <= 0 {
		return nil, text != ""
	}

	var lines []string
	var current []rune
	appendCurrent := func() {
		if len(current) > 0 {
			lines = append(lines, string(current))
			current = nil
		}
	}

	for _, word := range strings.Fields(text) {
		wordRunes := []rune(word)
		for len(wordRunes) > 0 {
			space := 0
			if len(current) > 0 {
				space = 1
			}
			remaining := maxChars - len(current) - space
			if remaining <= 0 {
				appendCurrent()
				continue
			}
			if len(wordRunes) <= remaining {
				if space == 1 {
					current = append(current, ' ')
				}
				current = append(current, wordRunes...)
				wordRunes = nil
				continue
			}
			if len(current) > 0 {
				appendCurrent()
				continue
			}
			current = append(current, wordRunes[:remaining]...)
			wordRunes = wordRunes[remaining:]
			appendCurrent()
		}
	}
	appendCurrent()

	if len(lines) <= maxLines {
		return lines, false
	}

	lines = lines[:maxLines]
	last := []rune(lines[maxLines-1])
	if maxChars >= 4 {
		if len(last) > maxChars-3 {
			last = last[:maxChars-3]
		}
		lines[maxLines-1] = strings.TrimSpace(string(last)) + "..."
	}
	return lines, true
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
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
