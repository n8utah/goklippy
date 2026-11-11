package camera

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Device is a capture device ffmpeg can open.
type Device struct {
	// Name is what Windows calls it ("USB Camera"). Identical cameras share it.
	Name string `json:"name"`
	// Path is DirectShow's device path ("@device_pnp_\\?\usb#vid_0bda&pid_5844&mi_00#
	// 7&3390689c&0&0000#{…}\global"). A camera without a serial number gets one from the
	// USB port it's in, so it stays the same while the camera stays in that port.
	// Empty with an old ffmpeg, which doesn't list it.
	Path string `json:"path,omitempty"`
	// ID is what to save to pick this camera again: Path when known, else Name.
	ID string `json:"id"`
	// Hint tells identical cameras apart: USB vendor:product and the port's instance
	// ("0bda:5844 · 3390689c").
	Hint string `json:"hint,omitempty"`
}

// DeviceList is what ListDevices found, and with which ffmpeg.
type DeviceList struct {
	Devices []Device `json:"devices"`
	FFmpeg  string   `json:"ffmpeg"`
	Version string   `json:"version,omitempty"`
	// Warning says why identical cameras can't be told apart (an old ffmpeg).
	Warning string `json:"warning,omitempty"`
}

// ResolveFFmpeg is the ffmpeg to run: the configured one, else on Windows winget's
// (installed with `winget install Gyan.FFmpeg`, which an older ffmpeg earlier on PATH
// would otherwise hide), else "ffmpeg" on PATH.
func ResolveFFmpeg(configured string) string {
	if configured != "" {
		return configured
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			link := filepath.Join(local, "Microsoft", "WinGet", "Links", "ffmpeg.exe")
			if _, err := os.Stat(link); err == nil {
				return link
			}
		}
	}
	return "ffmpeg"
}

// ListDevices asks ffmpeg for the video capture devices (DirectShow on Windows).
func ListDevices(ctx context.Context, configured string) (DeviceList, error) {
	ffmpeg := ResolveFFmpeg(configured)
	list := DeviceList{FFmpeg: ffmpeg, Devices: []Device{}}
	if runtime.GOOS != "windows" {
		return list, errors.New("listing devices is only done on Windows; give a /dev/videoN path")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, ffmpeg, args...)
		hideWindow(cmd)
		out, err := cmd.CombinedOutput() // listing always "fails": dummy isn't a device
		if len(out) == 0 && err != nil {
			return "", fmt.Errorf("run %s: %w", ffmpeg, err)
		}
		return string(out), nil
	}
	if out, err := run("-version"); err == nil {
		list.Version = versionLine(out)
	}
	out, err := run("-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	if err == nil && strings.Contains(out, "Unrecognized option") {
		out, err = run("-list_devices", "true", "-f", "dshow", "-i", "dummy") // an old ffmpeg
	}
	if err != nil {
		return list, err
	}
	list.Devices = ParseDshowDevices(out)
	if len(list.Devices) > 0 && list.Devices[0].Path == "" {
		list.Warning = "This ffmpeg is too old to list device paths, so identical cameras can't be told apart. " +
			"Install a current one (winget install Gyan.FFmpeg) or set its path below."
	}
	return list, nil
}

var ffmpegVersion = regexp.MustCompile(`ffmpeg version (\S+)`)

func versionLine(out string) string {
	if m := ffmpegVersion.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

var (
	// New ffmpeg: `[in#0 @ 000001] "Laptop Camera" (video)` then
	// `[in#0 @ 000001]   Alternative name "@device_pnp_…"`.
	dshowDevice = regexp.MustCompile(`"([^"]+)"\s+\((video|audio|none)\)`)
	dshowAlt    = regexp.MustCompile(`Alternative name\s+"([^"]+)"`)
	// Old ffmpeg lists "DirectShow video devices" then names, then audio devices.
	dshowHeading = regexp.MustCompile(`DirectShow (video|audio) devices`)
	dshowName    = regexp.MustCompile(`\]\s+"([^"]+)"\s*$`)
	// In a device path: usb#vid_0bda&pid_5844&mi_00#7&3390689c&0&0000#{…}
	pnpPath = regexp.MustCompile(`(?i)vid_([0-9a-f]{4})&pid_([0-9a-f]{4})[^#]*#([^#]+)#`)
)

// ParseDshowDevices pulls the video devices out of ffmpeg's -list_devices output.
func ParseDshowDevices(out string) []Device {
	var devices []Device
	seen := map[string]bool{}
	add := func(d Device) {
		d.ID = d.Path
		if d.ID == "" {
			d.ID = d.Name
		}
		if !seen[d.ID] {
			seen[d.ID] = true
			devices = append(devices, d)
		}
	}
	var pending *Device // a video device whose alternative name may follow
	flush := func() {
		if pending != nil {
			add(*pending)
			pending = nil
		}
	}
	section := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := dshowAlt.FindStringSubmatch(line); m != nil {
			if pending != nil {
				pending.Path = m[1]
				pending.Hint = pathHint(m[1])
			}
			flush()
			continue
		}
		if m := dshowDevice.FindStringSubmatch(line); m != nil {
			flush()
			if m[2] == "video" {
				pending = &Device{Name: m[1]}
			}
			continue
		}
		if m := dshowHeading.FindStringSubmatch(line); m != nil {
			flush()
			section = m[1]
			continue
		}
		if section == "video" {
			if m := dshowName.FindStringSubmatch(line); m != nil {
				add(Device{Name: m[1]})
			}
		}
	}
	flush()
	return devices
}

// Mode is a picture size and rate a camera offers.
type Mode struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Fps    float64 `json:"fps"`
	Format string  `json:"format"` // "mjpeg", "yuyv422", …
}

var dshowMode = regexp.MustCompile(`(?:vcodec|pixel_format)=(\S+)\s+min s=(\d+)x(\d+) fps=([\d.]+)`)

// ListModes asks a camera which sizes it offers. A camera already in use (by this
// backend or another program) may not answer.
func ListModes(ctx context.Context, configured, device string) ([]Mode, error) {
	if runtime.GOOS != "windows" {
		return nil, errors.New("listing modes is only done on Windows")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ResolveFFmpeg(configured), "-hide_banner", "-list_options", "true", "-f", "dshow", "-i", "video="+device)
	hideWindow(cmd)
	out, _ := cmd.CombinedOutput() // it always "fails": it lists and exits
	modes := ParseDshowModes(string(out))
	if len(modes) == 0 {
		return nil, fmt.Errorf("the camera didn't list its sizes: %s", lastLine(string(out)))
	}
	return modes, nil
}

// ParseDshowModes reads ffmpeg's -list_options output, one entry per size, format
// and rate.
func ParseDshowModes(out string) []Mode {
	var modes []Mode
	seen := map[Mode]bool{}
	for _, line := range strings.Split(out, "\n") {
		m := dshowMode.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var mode Mode
		mode.Format = m[1]
		fmt.Sscan(m[2], &mode.Width)
		fmt.Sscan(m[3], &mode.Height)
		fmt.Sscan(m[4], &mode.Fps)
		if !seen[mode] {
			seen[mode] = true
			modes = append(modes, mode)
		}
	}
	return modes
}

// pathHint shortens a device path to what tells cameras apart.
func pathHint(path string) string {
	m := pnpPath.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	instance := strings.Split(m[3], "&")
	port := m[3]
	if len(instance) > 1 {
		port = instance[1]
	}
	return fmt.Sprintf("%s:%s · %s", strings.ToLower(m[1]), strings.ToLower(m[2]), strings.ToLower(port))
}
