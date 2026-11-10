package sorter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memStore struct {
	docs map[string]json.RawMessage
	err  error
}

func (s *memStore) GetSorterSettings(_ context.Context, name string) (json.RawMessage, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	doc, ok := s.docs[name]
	return doc, ok, nil
}

func (s *memStore) PutSorterSettings(_ context.Context, name string, data json.RawMessage) error {
	if s.err != nil {
		return s.err
	}
	s.docs[name] = data
	return nil
}

func TestSettingsStartOnDefaults(t *testing.T) {
	svc := NewSettingsService(context.Background(), &memStore{docs: map[string]json.RawMessage{}})
	assert.Equal(t, DefaultMachineSettings(), svc.Machine())

	broken := NewSettingsService(context.Background(), &memStore{err: errors.New("no table")})
	assert.Equal(t, DefaultMachineSettings(), broken.Machine())
}

func TestSettingsLoadsSavedOverDefaults(t *testing.T) {
	store := &memStore{docs: map[string]json.RawMessage{
		SettingsMachine: json.RawMessage(`{"indexRpm": 12, "channels": {"c4": {"stepper": "stepper_e0", "forward": false, "gear": 10.83}}, "showSettings": true}`),
	}}
	got := NewSettingsService(context.Background(), store).Machine()
	assert.Equal(t, 12.0, got.IndexRpm)
	assert.Equal(t, 72.0, got.IndexDegrees, "missing fields keep defaults")
	assert.Equal(t, ChannelSettings{Stepper: "stepper_e0", Forward: false, Gear: 10.83}, got.Channels["c4"])
	assert.Equal(t, "stepper_x", got.Channels["c1"].Stepper, "unsaved channels keep defaults")
}

func TestSettingsPut(t *testing.T) {
	store := &memStore{docs: map[string]json.RawMessage{}}
	svc := NewSettingsService(context.Background(), store)
	ctx := context.Background()

	doc := json.RawMessage(`{"indexAccel": 90, "pageOnly": [1, 2]}`)
	require.NoError(t, svc.Put(ctx, SettingsMachine, doc))
	assert.Equal(t, 90.0, svc.Machine().IndexAccel)
	saved, ok, err := svc.Get(ctx, SettingsMachine)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.JSONEq(t, string(doc), string(saved), "fields the backend doesn't know are kept")

	require.NoError(t, svc.Put(ctx, SettingsRoutines, json.RawMessage(`[{"id": "r1"}]`)))

	assert.ErrorIs(t, svc.Put(ctx, SettingsMachine, json.RawMessage(`[]`)), ErrInvalidSettings)
	assert.ErrorIs(t, svc.Put(ctx, SettingsMachine, json.RawMessage(`{"indexRpm": "fast"}`)), ErrInvalidSettings)
	assert.ErrorIs(t, svc.Put(ctx, SettingsRoutines, json.RawMessage(`{}`)), ErrInvalidSettings)
	assert.ErrorIs(t, svc.Put(ctx, "other", json.RawMessage(`{}`)), ErrInvalidSettings)
	assert.Equal(t, 90.0, svc.Machine().IndexAccel, "a rejected save changes nothing")
}

func TestMoveDuration(t *testing.T) {
	// 72° at 10 rpm with no ramp: 0.2 turns at 1/6 turn per second.
	assert.InDelta(t, 1.2, moveDuration(72, 10, 0).Seconds(), 1e-9)
	// 60 rpm/s ramps to 10 rpm in 1/6 s each way, covering 1/72 turn each.
	assert.InDelta(t, 2.0/6+(0.2-2.0/72)*6, moveDuration(72, 10, 60).Seconds(), 1e-9)
	assert.InDelta(t, 1.2, moveDuration(-72, 10, 0).Seconds(), 1e-9, "direction doesn't matter")
	assert.Equal(t, time.Duration(0), moveDuration(0, 10, 60))
}

func TestSimHardwareSelectLayer(t *testing.T) {
	hw := NewSimHardware(DefaultMachineSettings)
	require.NoError(t, hw.SelectLayer(2))
	assert.Equal(t, 2, hw.Layer())
	assert.Error(t, hw.SelectLayer(4))
	assert.Error(t, hw.SelectLayer(-2))
}
