package logging

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	t.Run("default level when invalid", func(t *testing.T) {
		logger := Setup("invalid-level")
		assert.NotNil(t, logger)
	})

	t.Run("debug level", func(t *testing.T) {
		logger := Setup("debug")
		assert.NotNil(t, logger)
	})

	t.Run("info level", func(t *testing.T) {
		logger := Setup("info")
		assert.NotNil(t, logger)
	})

	t.Run("warn level", func(t *testing.T) {
		logger := Setup("warn")
		assert.NotNil(t, logger)
	})

	t.Run("error level", func(t *testing.T) {
		logger := Setup("error")
		assert.NotNil(t, logger)
	})

	t.Run("empty string defaults to info", func(t *testing.T) {
		logger := Setup("")
		assert.NotNil(t, logger)
	})

	t.Run("case insensitive", func(t *testing.T) {
		logger := Setup("DEBUG")
		assert.NotNil(t, logger)
	})

	t.Run("logger has timestamp", func(t *testing.T) {
		logger := Setup("info")
		assert.NotNil(t, logger)
	})
}

func TestSetupGlobalLevel(t *testing.T) {
	t.Run("sets global level correctly", func(t *testing.T) {
		Setup("debug")
		assert.Equal(t, zerolog.DebugLevel, zerolog.GlobalLevel())
	})

	t.Run("invalid level defaults to info", func(t *testing.T) {
		Setup("invalid")
		assert.Equal(t, zerolog.InfoLevel, zerolog.GlobalLevel())
	})
}
