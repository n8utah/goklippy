package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
)

func main() {
	port := flag.String("port", "", "serial port (e.g. COM5)")
	baud := flag.Int("baud", 250000, "serial baud rate")
	timeout := flag.Duration("timeout", mcu.DefaultProbeTimeout, "per-operation timeout")
	configPath := flag.String("config", "", "optional Klipper .cfg to translate and configure")
	jsonOut := flag.Bool("json", false, "print result as JSON")
	flag.Parse()

	if *port == "" {
		fmt.Fprintln(os.Stderr, "error: --port is required")
		flag.Usage()
		os.Exit(2)
	}

	if *configPath != "" {
		runConfigure(*port, *baud, *timeout, *configPath, *jsonOut)
		return
	}

	result, err := mcu.Probe(*port, *baud, *timeout)
	if err != nil {
		slog.Error("mcu probe failed", "port", *port, "error", err)
		os.Exit(1)
	}

	printProbeResult(result, *jsonOut)
}

func runConfigure(port string, baud int, timeout time.Duration, configPath string, jsonOut bool) {
	f, err := klipperconfig.LoadFile(configPath)
	if err != nil {
		slog.Error("load config failed", "path", configPath, "error", err)
		os.Exit(1)
	}
	lines, err := cfgtranslate.BuildHardwareCommands(f)
	if err != nil {
		slog.Error("translate config failed", "error", err)
		os.Exit(1)
	}

	conn, err := mcu.OpenConn(port, baud, timeout)
	if err != nil {
		slog.Error("open port failed", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.Configure(lines); err != nil {
		slog.Error("configure failed", "error", err)
		os.Exit(1)
	}

	status, err := conn.GetConfig()
	if err != nil {
		slog.Error("get_config failed", "error", err)
		os.Exit(1)
	}

	out := map[string]interface{}{
		"port":        port,
		"configured":  status.IsConfig,
		"crc":         status.CRC,
		"move_count":  status.MoveCount,
		"config_cmds": len(lines),
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}
	fmt.Println("MCU configure OK")
	fmt.Printf("  port=%s configured=%v crc=%d move_count=%d commands=%d\n",
		port, status.IsConfig, status.CRC, status.MoveCount, len(lines))
}

func printProbeResult(result mcu.IdentifyResult, jsonOut bool) {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	fmt.Println("Klipper MCU identify OK")
	fmt.Println(result)
	if len(result.Constants) > 0 {
		fmt.Println("\nMCU constants:")
		for k, v := range result.Constants {
			fmt.Printf("  %s = %v\n", k, v)
		}
	}
}
