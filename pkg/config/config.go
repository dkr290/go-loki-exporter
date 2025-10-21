// Package config - for configuration changes
package config

import (
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

type Config struct {
	LokiChunkSize    int
	NamespaceQuery   string
	LokiAddr         string
	ScheduleInterval int
	DirLogPaths      string
	Checkpoint       string
	DebugFlag        bool
	MaxQueryLogs     int
}

var nsQuery = `{namespace="namespace1", app!="api1", app!="api2"} |= "" `

func Load() *Config {
	return &Config{
		LokiChunkSize:    getEnv("LOKI_CHUNK_SIZE", 3),
		NamespaceQuery:   getEnv("NAMESPACE_QUERY", nsQuery),
		LokiAddr:         getEnv("LOKI_ADDR", "http://scaledloki-gateway:80"),
		ScheduleInterval: getEnv("SCHEDULE_INTERVAL", 15),
		DirLogPaths:      getEnv("DIR_LOG_PATHS", "/data/export"),
		Checkpoint:       getEnv("CHECKPOINT", "/data/checkpoint.json"),
		DebugFlag:        getEnv("DEBUGFLAG", false),
		MaxQueryLogs:     getEnv("MAXLOGSQUERY", 5000),
	}
}

type EnvType interface {
	string | int | bool
}

func getEnv[T EnvType](key string, defaultValue T) T {
	rawValue := os.Getenv(key)
	if rawValue == "" {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(rawValue).(T)

	case int:
		if i, err := strconv.Atoi(rawValue); err == nil {
			return any(i).(T)
		} else {
			log.Fatal().Err(err).Msg("Error converting the value in config")
		}
	case bool:
		if b, err := strconv.ParseBool(rawValue); err == nil {
			// Return the parsed bool if successful
			return any(b).(T)
		} else {
			log.Fatal().Err(err).Msg("Error converting the value in config")
		}

	}
	return defaultValue
}
