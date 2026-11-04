//go:build integration

package mcu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const integrationAPI = "http://127.0.0.1:8080/api/v1"

func TestStepperRoutineLiveAPI(t *testing.T) {
	port := os.Getenv("MCU_TEST_PORT")
	if port == "" {
		port = "COM6"
	}

	sessionID := integrationConnect(t, port)
	defer integrationPOST(t, "/mcu/disconnect", map[string]string{"session_id": sessionID}, nil)

	// Stop anything already running.
	integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/stop", sessionID), map[string]int{"oid": 0}, nil)

	body := map[string]interface{}{
		"oid": 0,
		"segments": []map[string]interface{}{
			{"forward": true, "speed_hz": 500, "steps": 50},
			{"forward": false, "speed_hz": 500, "steps": 10},
		},
	}
	integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/routine", sessionID), body, nil)

	// Pattern is ~120ms/rep; batch is several reps. Wait long enough for refill cycles.
	time.Sleep(3 * time.Second)

	// Jog should still work while routine runs on other oid — stop routine first.
	integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/stop", sessionID), map[string]int{"oid": 0}, nil)

	jogBody := map[string]interface{}{
		"oid":      0,
		"forward":  true,
		"speed_hz": 500,
		"steps":    20,
	}
	integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/jog", sessionID), jogBody, nil)
	integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/stop", sessionID), map[string]int{"oid": 0}, nil)
}

func TestStepperRoutineDualLiveAPI(t *testing.T) {
	port := os.Getenv("MCU_TEST_PORT")
	if port == "" {
		port = "COM6"
	}

	sessionID := integrationConnect(t, port)
	defer integrationPOST(t, "/mcu/disconnect", map[string]string{"session_id": sessionID}, nil)

	segments := []map[string]interface{}{
		{"forward": true, "speed_hz": 500, "steps": 50},
		{"forward": false, "speed_hz": 500, "steps": 10},
	}
	for _, oid := range []int{0, 2} {
		integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/stop", sessionID), map[string]int{"oid": oid}, nil)
		integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/routine", sessionID), map[string]interface{}{
			"oid":      oid,
			"segments": segments,
		}, nil)
	}

	// Run long enough for multiple refill cycles on both steppers.
	time.Sleep(8 * time.Second)

	for _, oid := range []int{0, 2} {
		integrationPOST(t, fmt.Sprintf("/mcu/sessions/%s/stepper/stop", sessionID), map[string]int{"oid": oid}, nil)
	}
}

func integrationConnect(t *testing.T, port string) string {
	t.Helper()
	var resp map[string]interface{}
	integrationPOST(t, "/mcu/connect", map[string]interface{}{
		"port":       port,
		"use_workspace": true,
		"profile_id": "ramps14",
	}, &resp)
	id, _ := resp["session_id"].(string)
	if id == "" {
		t.Fatalf("connect returned no session_id: %v", resp)
	}
	return id
}

func integrationPOST(t *testing.T, path string, body interface{}, out interface{}) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, integrationAPI+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		t.Fatalf("POST %s: status %d body %s", path, res.StatusCode, string(raw))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}
