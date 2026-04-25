package engine

import (
	"context"
	"testing"
)

func TestEngineConnectionType(t *testing.T) {
	var conn EngineConnection = context.Background()
	if conn == nil {
		t.Fatal("expected non-nil EngineConnection")
	}
}
