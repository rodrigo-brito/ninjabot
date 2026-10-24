package backtest

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	// the order controller logs every order at info level
	log.SetLevel(log.ErrorLevel)
	os.Exit(m.Run())
}
