package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/pkgs/labelmaker"
)

const zebraPrinterSettingsKey = "zebraPrinter"

// ZebraPrinterSettings mirrors the controls in dylanl321/zebra-label-maker.
type ZebraPrinterSettings struct {
	PrinterIP     string `json:"printerIp"`
	PrinterPort   int    `json:"printerPort"`
	LabelSize     string `json:"labelSize"`
	Orientation   string `json:"orientation"`
	Darkness      int    `json:"darkness"`
	PrintSpeed    int    `json:"printSpeed"`
	PrintFontSize int    `json:"printFontSize"`
}

func defaultZebraPrinterSettings(cfg *config.Config) ZebraPrinterSettings {
	return ZebraPrinterSettings{
		PrinterIP:     cfg.LabelMaker.PrinterIP,
		PrinterPort:   cfg.LabelMaker.PrinterPort,
		LabelSize:     cfg.LabelMaker.LabelSize,
		Orientation:   cfg.LabelMaker.Orientation,
		Darkness:      cfg.LabelMaker.PrintDarkness,
		PrintSpeed:    cfg.LabelMaker.PrintSpeed,
		PrintFontSize: cfg.LabelMaker.PrintFontSize,
	}
}

func decodeZebraPrinterSettings(raw interface{}, defaults ZebraPrinterSettings) ZebraPrinterSettings {
	data, err := json.Marshal(raw)
	if err != nil {
		return defaults
	}
	if err := json.Unmarshal(data, &defaults); err != nil {
		return defaults
	}
	return defaults
}

func validateZebraPrinterSettings(settings ZebraPrinterSettings) error {
	ip := net.ParseIP(strings.TrimSpace(settings.PrinterIP))
	if ip == nil {
		return errors.New("printer IP must be a valid IP address")
	}
	if !ip.IsPrivate() {
		return errors.New("printer IP must be on a private network")
	}
	if settings.PrinterPort < 1 || settings.PrinterPort > 65535 {
		return errors.New("printer port must be between 1 and 65535")
	}
	validSizes := map[string]bool{
		"1x1": true, "2x1": true, "2.25x1.25": true,
		"3x2": true, "4x2": true, "4x6": true,
	}
	if !validSizes[settings.LabelSize] {
		return errors.New("invalid label size")
	}
	if settings.Orientation != "portrait" && settings.Orientation != "landscape" {
		return errors.New("orientation must be portrait or landscape")
	}
	if settings.Darkness < 0 || settings.Darkness > 30 {
		return errors.New("darkness must be between 0 and 30")
	}
	if settings.PrintSpeed < 1 || settings.PrintSpeed > 14 {
		return errors.New("print speed must be between 1 and 14")
	}
	if settings.PrintFontSize < 20 || settings.PrintFontSize > 56 {
		return errors.New("font size must be between 20 and 56")
	}
	return nil
}

func applyZebraPrinterSettings(cfg *config.Config, settings ZebraPrinterSettings) *config.Config {
	effective := *cfg
	effective.LabelMaker = cfg.LabelMaker
	effective.LabelMaker.DirectPrint = true
	effective.LabelMaker.PrintServerURL = nil
	effective.LabelMaker.PrinterIP = strings.TrimSpace(settings.PrinterIP)
	effective.LabelMaker.PrinterPort = settings.PrinterPort
	effective.LabelMaker.LabelSize = settings.LabelSize
	effective.LabelMaker.Orientation = settings.Orientation
	effective.LabelMaker.PrintDarkness = settings.Darkness
	effective.LabelMaker.PrintSpeed = settings.PrintSpeed
	effective.LabelMaker.PrintFontSize = settings.PrintFontSize
	return &effective
}

func (ctrl *V1Controller) zebraPrinterSettingsForRequest(r *http.Request) (ZebraPrinterSettings, error) {
	actor := services.UseUserCtx(r.Context())
	defaults := defaultZebraPrinterSettings(ctrl.config)
	settings, err := ctrl.svc.User.GetSettings(r.Context(), actor.ID)
	if err != nil {
		return defaults, err
	}
	raw, ok := settings[zebraPrinterSettingsKey]
	if !ok {
		return defaults, nil
	}
	return decodeZebraPrinterSettings(raw, defaults), nil
}

// HandleZebraPrinterSettingsGet godoc
//
//	@Summary	Get Zebra printer settings
//	@Tags		Labelmaker
//	@Produce	json
//	@Success	200	{object}	ZebraPrinterSettings
//	@Router		/v1/labelmaker/settings [get]
//	@Security	Bearer
func (ctrl *V1Controller) HandleZebraPrinterSettingsGet() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		settings, err := ctrl.zebraPrinterSettingsForRequest(r)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}
		w.Header().Set("Cache-Control", "no-store")
		return server.JSON(w, http.StatusOK, settings)
	}
}

// HandleZebraPrinterSettingsUpdate godoc
//
//	@Summary	Update Zebra printer settings
//	@Tags		Labelmaker
//	@Accept		json
//	@Produce	json
//	@Param		payload	body		ZebraPrinterSettings	true	"Printer settings"
//	@Success	200		{object}	ZebraPrinterSettings
//	@Router		/v1/labelmaker/settings [put]
//	@Security	Bearer
func (ctrl *V1Controller) HandleZebraPrinterSettingsUpdate() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var input ZebraPrinterSettings
		if err := server.Decode(r, &input); err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}
		if err := validateZebraPrinterSettings(input); err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		actor := services.UseUserCtx(r.Context())
		settings, err := ctrl.svc.User.GetSettings(r.Context(), actor.ID)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}
		if settings == nil {
			settings = make(map[string]interface{})
		}
		settings[zebraPrinterSettingsKey] = input
		if err := ctrl.svc.User.SetSettings(r.Context(), actor.ID, settings); err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}
		return server.JSON(w, http.StatusOK, input)
	}
}

// HandleZebraPrinterTest godoc
//
//	@Summary	Print a Zebra test label
//	@Tags		Labelmaker
//	@Accept		json
//	@Produce	json
//	@Param		payload	body		ZebraPrinterSettings	true	"Printer settings"
//	@Success	200		{object}	map[string]bool
//	@Router		/v1/labelmaker/test [post]
//	@Security	Bearer
func (ctrl *V1Controller) HandleZebraPrinterTest() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var input ZebraPrinterSettings
		if err := server.Decode(r, &input); err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}
		if err := validateZebraPrinterSettings(input); err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		actor := services.UseUserCtx(r.Context())
		effective := applyZebraPrinterSettings(ctrl.config, input)
		hbURL := GetHBURL(r, &ctrl.config.Options, ctrl.url)
		params := labelmaker.NewGenerateParams(
			int(ctrl.config.LabelMaker.Width),
			int(ctrl.config.LabelMaker.Height),
			int(ctrl.config.LabelMaker.Margin),
			int(ctrl.config.LabelMaker.Padding),
			ctrl.config.LabelMaker.FontSize,
			"Homebox Printer Test",
			fmt.Sprintf("%s\n%s:%d", actor.Name, input.PrinterIP, input.PrinterPort),
			hbURL+"/profile",
			false,
			nil,
		)
		if err := labelmaker.PrintLabel(effective, &params); err != nil {
			return validate.NewRequestError(err, http.StatusBadGateway)
		}
		return server.JSON(w, http.StatusOK, map[string]bool{"printed": true})
	}
}
