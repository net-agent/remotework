package utils

import (
	"os"
	"strings"
	"testing"
)

func TestResolveSecret(t *testing.T) {
	t.Run("empty ref", func(t *testing.T) {
		got, err := ResolveSecret("")
		if err != nil || got != "" {
			t.Fatalf("ResolveSecret(\"\") = %q, %v; want \"\", nil", got, err)
		}
	})

	t.Run("bare env name", func(t *testing.T) {
		t.Setenv("RW_TEST_SECRET", "s3cr3t")
		got, err := ResolveSecret("RW_TEST_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "s3cr3t" {
			t.Fatalf("got %q, want s3cr3t", got)
		}
	})

	t.Run("env prefix", func(t *testing.T) {
		t.Setenv("RW_TEST_SECRET", "s3cr3t")
		got, err := ResolveSecret("env:RW_TEST_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "s3cr3t" {
			t.Fatalf("got %q, want s3cr3t", got)
		}
	})

	t.Run("unset env fails closed", func(t *testing.T) {
		t.Setenv("RW_TEST_UNSET", "placeholder")
		if err := os.Unsetenv("RW_TEST_UNSET"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
		got, err := ResolveSecret("RW_TEST_UNSET")
		if err == nil {
			t.Fatalf("expected error for unset env, got %q", got)
		}
		if !strings.Contains(err.Error(), "not set") {
			t.Fatalf("error should say not set, got %v", err)
		}
	})

	t.Run("empty env value fails closed", func(t *testing.T) {
		t.Setenv("RW_TEST_EMPTY", "")
		got, err := ResolveSecret("env:RW_TEST_EMPTY")
		if err == nil {
			t.Fatalf("expected error for empty env, got %q", got)
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Fatalf("error should say empty, got %v", err)
		}
	})

	t.Run("unsupported scheme", func(t *testing.T) {
		_, err := ResolveSecret("file:/tmp/secret")
		if err == nil {
			t.Fatal("expected error for unsupported scheme")
		}
		if !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("error should say unsupported, got %v", err)
		}
	})

	t.Run("env prefix with empty name", func(t *testing.T) {
		if _, err := ResolveSecret("env:"); err == nil {
			t.Fatal("expected error for empty env name")
		}
	})
}
