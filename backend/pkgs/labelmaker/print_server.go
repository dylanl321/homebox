package labelmaker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

type printServerRequest struct {
	ZPL  string `json:"zpl"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

func printerAddr(cfg *config.Config) (ip string, port int) {
	ip = "10.0.1.161"
	port = 9100
	if cfg == nil {
		return ip, port
	}
	if s := strings.TrimSpace(cfg.LabelMaker.PrinterIP); s != "" {
		ip = s
	}
	if cfg.LabelMaker.PrinterPort > 0 {
		port = cfg.LabelMaker.PrinterPort
	}
	return ip, port
}

// printZPL sends a Homebox label to a Zebra printer.
// Default path: raw TCP to PrinterIP:PrinterPort (same as zebra-label-maker's print-server.py).
// Optional: if PrintServerURL is set, POST ZPL through that HTTP bridge instead.
func printZPL(cfg *config.Config, params *GenerateParameters) error {
	zpl := GenerateZPL(params, cfg)
	if cfg != nil && cfg.LabelMaker.PrintServerURL != nil {
		if base := strings.TrimSpace(*cfg.LabelMaker.PrintServerURL); base != "" {
			return printViaHTTPBridge(cfg, zpl)
		}
	}
	return printViaTCP(cfg, zpl)
}

// printViaTCP writes ZPL directly to the printer over raw TCP (port 9100).
// No separate print-server process is required — Homebox is the client.
func printViaTCP(cfg *config.Config, zpl string) error {
	ip, port := printerAddr(cfg)
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect to printer %s: %w", addr, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(conn, zpl); err != nil {
		return fmt.Errorf("send ZPL to printer %s: %w", addr, err)
	}
	return nil
}

// printViaHTTPBridge is optional — only if something else already runs the
// zebra-label-maker HTTP bridge and Homebox cannot reach the printer directly.
func printViaHTTPBridge(cfg *config.Config, zpl string) error {
	base := strings.TrimRight(strings.TrimSpace(*cfg.LabelMaker.PrintServerURL), "/")
	printURL := base + "/print"
	ip, port := printerAddr(cfg)

	body, err := json.Marshal(printServerRequest{ZPL: zpl, IP: ip, Port: port})
	if err != nil {
		return fmt.Errorf("marshal print request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPost, printURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create print request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Homebox-LabelMaker/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("print server request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("print server returned %d: %s", resp.StatusCode, msg)
	}
	return nil
}
