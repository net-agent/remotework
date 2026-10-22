package netflex

import (
	"strings"
	"testing"
)

func TestResolveAuth(t *testing.T) {
	t.Run("plain auth without ref", func(t *testing.T) {
		got, err := resolveAuth("plain-secret", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "plain-secret" {
			t.Fatalf("got %q, want plain-secret", got)
		}
	})

	t.Run("authRef resolved from env", func(t *testing.T) {
		t.Setenv("RW_NETFLEX_AUTH", "resolved-secret")
		got, err := resolveAuth("ignored", "env:RW_NETFLEX_AUTH")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "resolved-secret" {
			t.Fatalf("got %q, want resolved-secret", got)
		}
	})

	t.Run("bare authRef resolved from env", func(t *testing.T) {
		t.Setenv("RW_NETFLEX_AUTH", "resolved-secret")
		got, err := resolveAuth("", "RW_NETFLEX_AUTH")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "resolved-secret" {
			t.Fatalf("got %q, want resolved-secret", got)
		}
	})

	t.Run("missing authRef fails closed", func(t *testing.T) {
		got, err := resolveAuth("plain", "RW_NETFLEX_MISSING")
		if err == nil {
			t.Fatal("expected error for missing authRef")
		}
		if got != "" {
			t.Fatalf("must not fall back to ref literal, got %q", got)
		}
		if strings.Contains(got, "RW_NETFLEX_MISSING") {
			t.Fatalf("ref literal leaked as auth: %q", got)
		}
	})
}
