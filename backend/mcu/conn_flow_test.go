package mcu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAckThroughDropsOnlyConsumedBlocks(t *testing.T) {
	c := &Conn{inflight: []sentBlock{{seq: 3, size: 10}, {seq: 4, size: 10}, {seq: 5, size: 10}}}

	c.ackThrough(4)
	assert.Equal(t, []sentBlock{{seq: 4, size: 10}, {seq: 5, size: 10}}, c.inflight)

	c.ackThrough(4) // a repeated ack changes nothing
	assert.Len(t, c.inflight, 2)

	c.ackThrough(6) // the next sequence after the last block means all were consumed
	assert.Empty(t, c.inflight)
}

func TestAckThroughHandlesSequenceWrap(t *testing.T) {
	c := &Conn{inflight: []sentBlock{{seq: 15, size: 8}, {seq: 0, size: 8}, {seq: 1, size: 8}}}

	c.ackThrough(0)
	assert.Equal(t, []sentBlock{{seq: 0, size: 8}, {seq: 1, size: 8}}, c.inflight)
}

func TestJogKeeperWaitStaysAheadOfTheQueue(t *testing.T) {
	for _, speedHz := range []float64{1, 100, 500, 5000, 50000} {
		batch := float64(maxQueueStepCount) / speedHz
		assert.Less(t, jogKeeperWait(speedHz).Seconds(), batch, "speed %v", speedHz)
	}
}
