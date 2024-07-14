package tools_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/tools"
)

func TestNewTrailingStop(t *testing.T) {
	ts := tools.NewTrailingStop()

	require.NotNil(t, ts)
}

func TestTrailingStop_StartError(t *testing.T) {
	ts := tools.NewTrailingStop()

	err := ts.Start(model.SideTypeBuy, 21.5, 13.0)
	require.NoError(t, err)

	err = ts.Start(model.SideTypeBuy, 21.5, 22)
	require.Error(t, err)

	err = ts.Start(model.SideTypeSell, 10, 13.0)
	require.NoError(t, err)

	err = ts.Start(model.SideTypeSell, 21.5, 13.0)
	require.Error(t, err)
}

func TestTrailingStop_Start(t *testing.T) {
	ts := tools.NewTrailingStop()
	err := ts.Start(model.SideTypeBuy, 21.5, 13.0)
	require.NoError(t, err)

	require.True(t, ts.Active())
}

func TestTrailingStop_Stop(t *testing.T) {
	ts := tools.NewTrailingStop()
	err := ts.Start(model.SideTypeBuy, 21.5, 13.0)
	require.NoError(t, err)

	ts.Stop()

	require.False(t, ts.Active())
}

func TestTrailingStop_UpdateLong(t *testing.T) {
	ts := tools.NewTrailingStop()

	// not started
	require.False(t, ts.Update(12.0))

	current := 21.5
	stop := 13.0

	err := ts.Start(model.SideTypeBuy, current, stop)
	require.NoError(t, err)

	// When the new value is higher than the current value, the TrailingStop is
	// not triggered and the stop value e summed up with the difference of the
	// two values.
	difference := 5.0
	require.False(t, ts.Update(current+difference))

	// So when called with the new stop value or a lower one, the TrailingStop
	// should be triggered.
	require.True(t, ts.Update(stop+difference))
	require.True(t, ts.Update(stop-difference))
}

func TestTrailingStop_UpdateShort(t *testing.T) {
	ts := tools.NewTrailingStop()

	// not started
	require.False(t, ts.Update(12.0))

	current := 13.0
	stop := 21.5

	err := ts.Start(model.SideTypeSell, current, stop)
	require.NoError(t, err)

	// When the new value is higher than the current value, the TrailingStop is
	// not triggered and the stop value e summed up with the difference of the
	// two values.
	difference := 5.0
	require.False(t, ts.Update(current-difference))

	// So when called with the new stop value or a lower one, the TrailingStop
	// should be triggered.
	require.True(t, ts.Update(stop-difference))
	require.True(t, ts.Update(stop+difference))
}
