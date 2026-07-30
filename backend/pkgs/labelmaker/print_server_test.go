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

func TestGenerateZPL_MatchesZebraLabelMaker2x1(t *testing.T) {
	params := NewGenerateParams(526, 200, 32, 32, 32, "Spare Parts", "Location: Garage", "http://homebox.local/item/abc", false, nil)
	cfg := &config.Config{}
	cfg.LabelMaker.LabelSize = "2x1"
	cfg.LabelMaker.Orientation = "portrait"
	cfg.LabelMaker.PrintDarkness = 15
	cfg.LabelMaker.PrintSpeed = 4
	cfg.LabelMaker.PrintFontSize = 30

	zpl := GenerateZPL(&params, cfg)

	assert.Contains(t, zpl, "^PW406")
	assert.Contains(t, zpl, "^LL203")
	assert.Contains(t, zpl, "^BQN,2,5^FDQA,http://homebox.local/item/abc^FS")
	assert.Contains(t, zpl, "^FB406,1,0,C^FDSpare Parts^FS")
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
	assert.Contains(t, zpl, "^BQN,2,5^FDQA,http://example/item/1^FS")
}

func TestPrintViaHTTPBridge_Optional(t *testing.T) {
	var gotBody printServerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
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
