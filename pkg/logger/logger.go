// Package logger
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
	Debug(msg string)
	Warn(msg string)
	// Add more as needed
}

type ZeroLogger struct {
	l zerolog.Logger
}

func (z ZeroLogger) Info(msg string)  { z.l.Info().Msg(msg) }
func (z ZeroLogger) Error(msg string) { z.l.Error().Msg(msg) }
func (z ZeroLogger) Debug(msg string) { z.l.Debug().Msg(msg) }
func (z ZeroLogger) Warn(msg string)  { z.l.Warn().Msg(msg) }

func New(debug bool) Logger {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)

	zl := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger()

	return ZeroLogger{l: zl}
}
