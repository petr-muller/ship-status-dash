package e2e

import (
	"testing"

	"ship-status-dash/pkg/config"
)

func TestConfigReloadObservedSincePatch(t *testing.T) {
	t.Parallel()
	line := `time="2026-01-02T15:04:05Z" level=info msg="` + config.ConfigReloadedMessage + `" config_path=/x reload_count=4`
	t.Run("counter increased", func(t *testing.T) {
		if !configReloadObservedSincePatch(line, 3) {
			t.Fatal("expected true when reload_count > baseline")
		}
	})
	t.Run("counter not increased", func(t *testing.T) {
		if configReloadObservedSincePatch(line, 4) {
			t.Fatal("expected false when reload_count == baseline")
		}
	})
	t.Run("success line without reload_count", func(t *testing.T) {
		legacy := `level=info msg="` + config.ConfigReloadedMessage + `" config_path=/x`
		if configReloadObservedSincePatch(legacy, 0) {
			t.Fatal("expected false without reload_count on the success line")
		}
	})
}
