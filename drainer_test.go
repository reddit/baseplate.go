package baseplate_test

import (
	"context"
	"testing"

	baseplate "github.com/reddit/baseplate.go"
)

func TestDrainer(t *testing.T) {
	ctx := context.Background()
	drainer := baseplate.Drainer()
	if !drainer.IsHealthy(ctx) {
		t.Error("Drainer does not report healthy after created")
	}
	if err := drainer.Close(); err != nil {
		t.Errorf("Drainer.Close did not expect error, got %v", err)
	}
	if drainer.IsHealthy(ctx) {
		t.Error("Drainer reports healthy after Close was called")
	}
	// Make sure double close works.
	if err := drainer.Close(); err != nil {
		t.Errorf("Drainer.Close did not expect error, got %v", err)
	}
	if drainer.IsHealthy(ctx) {
		t.Error("Drainer reports healthy after Close was called")
	}
}
