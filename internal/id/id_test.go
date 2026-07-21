package id

import (
	"regexp"
	"testing"
)

func TestNewRunIDReturnsUUIDv4(t *testing.T) {
	value, err := NewRunID()
	if err != nil {
		t.Fatalf("NewRunID() error = %v", err)
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(value) {
		t.Fatalf("NewRunID() = %q, want RFC 4122 UUIDv4", value)
	}
}

func TestNewRunIDIsNotConstant(t *testing.T) {
	first, err := NewRunID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRunID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("two generated run IDs are equal: %q", first)
	}
}
