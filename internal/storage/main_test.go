package storage

import (
	"os"
	"testing"

	"scout-app/internal/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.StartDB()
	os.Exit(m.Run())
}
