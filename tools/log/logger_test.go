package log

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// captureLog redirects logrus to a buffer for the duration of fn and returns
// everything that was written.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()

	logger := logrus.StandardLogger()
	previousOut, previousLevel, previousExit := logger.Out, logger.Level, logger.ExitFunc

	var buffer bytes.Buffer
	logger.Out = &buffer
	logger.Level = logrus.DebugLevel
	logger.ExitFunc = func(int) {} // keep Fatal from killing the test binary

	t.Cleanup(func() {
		logger.Out, logger.Level, logger.ExitFunc = previousOut, previousLevel, previousExit
	})

	fn()

	return buffer.String()
}

func TestLog(t *testing.T) {
	tests := []struct {
		name      string
		level     Level
		wantLevel string
	}{
		{"info", InfoLevel, "level=info"},
		{"warn", WarnLevel, "level=warning"},
		{"error", ErrorLevel, "level=error"},
		{"fatal", FatalLevel, "level=fatal"},
		{"debug", DebugLevel, "level=debug"},
		{"unknown levels fall back to debug", logrus.TraceLevel, "level=debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureLog(t, func() {
				Log(tt.level, "message")
			})

			require.Contains(t, output, tt.wantLevel)
			require.Contains(t, output, "message")
		})
	}

	t.Run("panic", func(t *testing.T) {
		output := captureLog(t, func() {
			require.Panics(t, func() { Log(PanicLevel, "message") })
		})

		require.Contains(t, output, "level=panic")
	})
}

func TestCheckErr(t *testing.T) {
	t.Run("logs the error when there is one", func(t *testing.T) {
		output := captureLog(t, func() {
			CheckErr(ErrorLevel, errors.New("boom"))
		})

		require.Contains(t, output, "level=error")
		require.Contains(t, output, "boom")
	})

	t.Run("stays quiet on a nil error", func(t *testing.T) {
		output := captureLog(t, func() {
			CheckErr(ErrorLevel, nil)
		})

		require.Empty(t, output)
	})
}

func TestLevelHelpers(t *testing.T) {
	tests := []struct {
		name      string
		log       func()
		wantLevel string
		wantText  string
	}{
		{"Info", func() { Info("hello") }, "level=info", "hello"},
		{"Infof", func() { Infof("hello %s", "world") }, "level=info", "hello world"},
		{"Warn", func() { Warn("hello") }, "level=warning", "hello"},
		{"Warnf", func() { Warnf("hello %s", "world") }, "level=warning", "hello world"},
		{"Error", func() { Error("hello") }, "level=error", "hello"},
		{"Errorf", func() { Errorf("hello %s", "world") }, "level=error", "hello world"},
		{"Fatal", func() { Fatal("hello") }, "level=fatal", "hello"},
		{"Debug", func() { Debug("hello") }, "level=debug", "hello"},
		{"Debugf", func() { Debugf("hello %s", "world") }, "level=debug", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureLog(t, tt.log)

			require.Contains(t, output, tt.wantLevel)
			require.Contains(t, output, tt.wantText)
		})
	}
}

func TestWithFields(t *testing.T) {
	t.Run("WithField", func(t *testing.T) {
		output := captureLog(t, func() {
			WithField("pair", "BTCUSDT").Info("order")
		})

		require.Contains(t, output, "pair=BTCUSDT")
	})

	t.Run("WithFields", func(t *testing.T) {
		output := captureLog(t, func() {
			WithFields(logrus.Fields{"pair": "BTCUSDT", "side": "BUY"}).Info("order")
		})

		require.Contains(t, output, "pair=BTCUSDT")
		require.Contains(t, output, "side=BUY")
	})
}

func TestSetLevel(t *testing.T) {
	previous := logrus.GetLevel()
	t.Cleanup(func() { SetLevel(previous) })

	output := captureLog(t, func() {
		SetLevel(ErrorLevel)
		Info("filtered out")
		Error("kept")
	})

	require.NotContains(t, output, "filtered out")
	require.Contains(t, output, "kept")
}

func TestSetFormatter(t *testing.T) {
	previous := logrus.StandardLogger().Formatter
	t.Cleanup(func() { SetFormatter(previous) })

	output := captureLog(t, func() {
		SetFormatter(&TextFormatter{DisableTimestamp: true, DisableColors: true})
		Info("formatted")
	})

	require.Equal(t, "level=info msg=formatted\n", output)
}
