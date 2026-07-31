package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

func TestValidateZebraPrinterSettings(t *testing.T) {
	valid := ZebraPrinterSettings{
		PrinterIP:     "10.0.1.161",
		PrinterPort:   9100,
		LabelSize:     "2x1",
		Orientation:   "portrait",
		Darkness:      15,
		PrintSpeed:    4,
		PrintFontSize: 30,
	}
	require.NoError(t, validateZebraPrinterSettings(valid))

	invalid := valid
	invalid.PrinterIP = "8.8.8.8"
	require.ErrorContains(t, validateZebraPrinterSettings(invalid), "private network")

	invalid = valid
	invalid.LabelSize = "5x7"
	require.ErrorContains(t, validateZebraPrinterSettings(invalid), "invalid label size")
}

func TestApplyZebraPrinterSettings(t *testing.T) {
	serverURL := "http://bridge:5555"
	cfg := &config.Config{}
	cfg.LabelMaker.PrintServerURL = &serverURL

	settings := ZebraPrinterSettings{
		PrinterIP:     "10.0.1.161",
		PrinterPort:   9100,
		LabelSize:     "2x1",
		Orientation:   "landscape",
		Darkness:      18,
		PrintSpeed:    5,
		PrintFontSize: 40,
	}

	effective := applyZebraPrinterSettings(cfg, settings)
	assert.True(t, effective.LabelMaker.DirectPrint)
	assert.Nil(t, effective.LabelMaker.PrintServerURL)
	assert.Equal(t, "10.0.1.161", effective.LabelMaker.PrinterIP)
	assert.Equal(t, "landscape", effective.LabelMaker.Orientation)
	assert.Equal(t, 18, effective.LabelMaker.PrintDarkness)
	assert.NotNil(t, cfg.LabelMaker.PrintServerURL, "source config must remain unchanged")
}
