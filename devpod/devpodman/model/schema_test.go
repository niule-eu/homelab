package model

import "testing"

func TestSchemaNotEmpty(t *testing.T) {
	if Schema == "" {
		t.Fatal("embedded schema is empty")
	}
}
