//go:build linux && (amd64 || arm64)

package podtoast

import (
	"context"
	"os"
	"testing"
)

func TestRetireRefusesUnmanagedRoot(t *testing.T) {
	if _, err := os.Stat("/podtoast"); err == nil {
		t.Skip("managed toolbox")
	}
	if err := RetireIdle(context.Background()); err == nil {
		t.Fatal("retirement accepted outside a verified toolbox")
	}
}
