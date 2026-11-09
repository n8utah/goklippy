package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeProbeResultsKeepsLiveDetailsAndDropsMissingPorts(t *testing.T) {
	ports := []PortInfo{
		{Name: "COM6", Description: "USB-SERIAL CH340", IsUSB: true},
		{Name: "COM7", Description: "Bluetooth link"},
	}
	probed := []PortInfo{
		{Name: "COM6", Description: "stale", IsKlipper: true, Version: "v0.12", MessageCount: 90},
		{Name: "COM9", IsKlipper: true},
	}

	mergeProbeResults(ports, probed)

	assert.Equal(t, []PortInfo{
		{Name: "COM6", Description: "USB-SERIAL CH340", IsUSB: true, IsKlipper: true, Version: "v0.12", MessageCount: 90},
		{Name: "COM7", Description: "Bluetooth link"},
	}, ports)
}
