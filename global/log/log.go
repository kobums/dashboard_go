package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dashboard/global/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	ConsoleLoggingEnabled bool
	FileLoggingEnabled    bool
	Directory             string
	Filename              string
	MaxSize               int
	MaxBackups            int
	MaxAge                int
}

var _log *zerolog.Logger

var _lumberjack *lumberjack.Logger

func Rotate() {
	err := _lumberjack.Rotate()
	if err != nil {
		log.Error().Msg(err.Error())
	}
}

func init() {
	fmt.Println("log.go init")
	config.Init()

	var writers []io.Writer

	if config.Log.Console {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
	}

	if config.Log.File != "" {
		directory := filepath.Dir(config.Log.File)

		if err := os.MkdirAll(directory, 0755); err != nil {
			log.Error().Err(err).Str("path", directory).Msg("can't create log directory")
			return
		}

		_lumberjack = &lumberjack.Logger{
			LocalTime:        true,
			Filename:         config.Log.File,
			MaxBackups:       config.Log.Limit.Count,
			MaxSize:          config.Log.Limit.Size,
			MaxAge:           config.Log.Limit.Days,
		}

		writers = append(writers, _lumberjack)
	}

	mw := zerolog.MultiLevelWriter(writers...)

	level := strings.ToLower(config.Log.Level)
	if level == "info" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else if level == "warn" || level == "warning" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else if level == "err" || level == "error" {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	logger := zerolog.New(mw).With().Timestamp().Logger()

	_log = &logger
	/*
		_log = &Logger{
			Logger: &logger,
		}
	*/
}

func Println(a ...any) {
	str := ""
	for i, v := range a {
		if i > 0 {
			str += " "
		}
		str += fmt.Sprintf("%v", v)
	}
	_log.Debug().Msg(str)
}

func Get() *zerolog.Logger {
	return _log
}

func Printf(format string, a ...any) {
	_log.Debug().Msgf(format, a...)
}

func Debug() *zerolog.Event {
	return _log.Debug()
}

func Info() *zerolog.Event {
	return _log.Info()
}

func Warn() *zerolog.Event {
	return _log.Warn()
}

func Error() *zerolog.Event {
	return _log.Error()
}
