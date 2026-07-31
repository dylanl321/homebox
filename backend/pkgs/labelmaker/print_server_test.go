package labelmaker

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

func TestGenerateZPL_UsesBoundedSideBySide2x1Layout(t *testing.T) {
	params := NewGenerateParams(526, 200, 32, 32, 32, "Spare Parts", "Location: Garage", "http://homebox.local/item/abc", false, nil)
	cfg := &config.Config{}
	cfg.LabelMaker.LabelSize = "2x1"
	cfg.LabelMaker.Orientation = "portrait"
	cfg.LabelMaker.PrintDarkness = 15
	cfg.LabelMaker.PrintSpeed = 4
	cfg.LabelMaker.PrintFontSize = 30

	zpl := GenerateZPL(&params, cfg)
	layout := buildZPLLayout(&params, cfg)

	assert.Contains(t, zpl, "^PW406")
	assert.Contains(t, zpl, "^LL203")
	assert.Contains(t, zpl, "^BQN,2,")
	assert.Contains(t, zpl, "^FDQA,http://homebox.local/item/abc^FS")
	assert.Less(t, layout.QRX+layout.QRSize, layout.TextX, "QR region must end before text begins")
	assert.LessOrEqual(t, layout.TextX+layout.TextWidth, layout.Width-layout.Margin)
	assert.NotEmpty(t, layout.TextLines)
	for _, line := range layout.TextLines {
		assert.GreaterOrEqual(t, line.Y, layout.Margin)
		assert.LessOrEqual(t, line.Y+line.Font, layout.Height-layout.Margin)
	}
}

func TestGenerateZPL_TruncatesLongTextWithoutOverlap(t *testing.T) {
	longTitle := strings.Repeat("Extremely Long Inventory Item Name ", 8)
	longDescription := strings.Repeat("Location and descriptive information that must remain bounded ", 12)
	params := NewGenerateParams(526, 200, 32, 32, 32, longTitle, longDescription, "http://homebox.local/item/very-long-identifier", false, nil)
	cfg := &config.Config{}
	cfg.LabelMaker.LabelSize = "2x1"
	cfg.LabelMaker.PrintFontSize = 30

	layout := buildZPLLayout(&params, cfg)
	zpl := GenerateZPL(&params, cfg)

	assert.Contains(t, zpl, "...")
	assert.NotContains(t, zpl, longTitle)
	assert.Less(t, layout.QRX+layout.QRSize, layout.TextX)
	for _, line := range layout.TextLines {
		assert.LessOrEqual(t, line.Y+line.Font, layout.Height-layout.Margin)
	}
}

func TestZPLLayoutStaysBoundedForEveryPresetAndOrientation(t *testing.T) {
	params := NewGenerateParams(
		526,
		200,
		32,
		32,
		32,
		strings.Repeat("Long inventory title ", 6),
		strings.Repeat("Detailed location and item information ", 10),
		"http://homebox.local/item/1234567890",
		false,
		nil,
	)

	for _, size := range []string{"1x1", "2x1", "2.25x1.25", "3x2", "4x2", "4x6"} {
		for _, orientation := range []string{"portrait", "landscape"} {
			t.Run(size+"-"+orientation, func(t *testing.T) {
				cfg := &config.Config{}
				cfg.LabelMaker.LabelSize = size
				cfg.LabelMaker.Orientation = orientation
				cfg.LabelMaker.PrintFontSize = 30

				layout := buildZPLLayout(&params, cfg)

				assert.Greater(t, layout.QRX, 0)
				assert.Greater(t, layout.QRY, 0)
				assert.Less(t, layout.QRX+layout.QRSize, layout.TextX)
				assert.LessOrEqual(t, layout.TextX+layout.TextWidth, layout.Width-layout.Margin)
				for _, line := range layout.TextLines {
					assert.GreaterOrEqual(t, line.Y, layout.Margin)
					assert.LessOrEqual(t, line.Y+line.Font, layout.Height-layout.Margin)
				}
			})
		}
	}
}

func TestGenerateZPL_Landscape2x1Is1x2(t *testing.T) {
	params := NewGenerateParams(526, 200, 32, 32, 32, "T", "", "http://x", false, nil)
	cfg := &config.Config{}
	cfg.LabelMaker.LabelSize = "2x1"
	cfg.LabelMaker.Orientation = "landscape"

	zpl := GenerateZPL(&params, cfg)
	assert.Contains(t, zpl, "^PW203")
	assert.Contains(t, zpl, "^LL406")
}

func TestPrintViaTCP_SendsZPL(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		raw, _ := io.ReadAll(conn)
		got <- string(raw)
	}()

	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	cfg := &config.Config{}
	cfg.LabelMaker.DirectPrint = true
	cfg.LabelMaker.PrinterIP = "127.0.0.1"
	cfg.LabelMaker.PrinterPort = port
	cfg.LabelMaker.LabelSize = "2x1"

	params := NewGenerateParams(526, 200, 32, 32, 32, "Box A", "", "http://example/item/1", false, nil)
	require.NoError(t, PrintLabel(cfg, &params))

	zpl := <-got
	assert.True(t, strings.HasPrefix(zpl, "^XA"))
	assert.Contains(t, zpl, "Box A")
	assert.Contains(t, zpl, "^BQN,2,")
	assert.Contains(t, zpl, "^FDQA,http://example/item/1^FS")
}

func TestPrintViaHTTPBridge_Optional(t *testing.T) {
	var gotBody printServerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		if !assert.NoError(t, json.Unmarshal(raw, &gotBody)) {
			http.Error(w, "decode failed", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	url := srv.URL
	cfg := &config.Config{}
	cfg.LabelMaker.DirectPrint = false
	cfg.LabelMaker.PrintServerURL = &url
	cfg.LabelMaker.PrinterIP = "10.0.1.161"
	cfg.LabelMaker.PrinterPort = 9100

	params := NewGenerateParams(526, 200, 32, 32, 32, "Box A", "", "http://example/item/1", false, nil)
	require.NoError(t, PrintLabel(cfg, &params))
	assert.Equal(t, "10.0.1.161", gotBody.IP)
	assert.Contains(t, gotBody.ZPL, "Box A")
}

func TestPrintLabel_RequiresPrintMethod(t *testing.T) {
	cfg := &config.Config{}
	cfg.LabelMaker.DirectPrint = false
	params := NewGenerateParams(400, 200, 10, 5, 32, "t", "d", "http://x", false, nil)
	err := PrintLabel(cfg, &params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no print method configured")
}

func TestPrintingEnabled(t *testing.T) {
	cfg := config.LabelMakerConf{DirectPrint: true}
	assert.True(t, cfg.PrintingEnabled())

	cfg = config.LabelMakerConf{DirectPrint: false}
	assert.False(t, cfg.PrintingEnabled())

	url := "http://127.0.0.1:5555"
	cfg.PrintServerURL = &url
	assert.True(t, cfg.PrintingEnabled())
}
