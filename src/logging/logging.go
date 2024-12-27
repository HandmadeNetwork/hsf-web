package logging

import (
	"context"
	"encoding/json"
	"hsf/src/config"
	"hsf/src/ee"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// See this file for a good color reference:
// https://github.com/fatih/color/blob/main/color.go

var Reset = "\033[0m"
var Bold = "\033[1m"
var Faint = "\033[2m"
var Italic = "\033[3m"
var Underline = "\033[4m"
var BlinkSlow = "\033[5m"
var BlinkRapid = "\033[6m"
var ReverseVideo = "\033[7m"
var Concealed = "\033[8m"
var CrossedOut = "\033[9m"

var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Purple = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

var BgBlack = "\033[40m"
var BgRed = "\033[41m"
var BgGreen = "\033[42m"
var BgYellow = "\033[43m"
var BgBlue = "\033[44m"
var BgMagenta = "\033[45m"
var BgCyan = "\033[46m"
var BgWhite = "\033[47m"

func init() {
	if runtime.GOOS == "windows" {
		Reset = BgBlack + Reset
	}
}

func init() {
	zerolog.ErrorStackMarshaler = ee.ZerologStackMarshaler
	log.Logger = log.Output(NewPrettyZerologWriter()).With().Stack().Logger()
	zerolog.SetGlobalLevel(config.Config.LogLevel)
}

func GlobalLogger() *zerolog.Logger {
	return &log.Logger
}

func Trace() *zerolog.Event {
	return log.Trace().Timestamp().Stack()
}

func Debug() *zerolog.Event {
	return log.Debug().Timestamp().Stack()
}

func Info() *zerolog.Event {
	return log.Info().Timestamp().Stack()
}

func Warn() *zerolog.Event {
	return log.Warn().Timestamp().Stack()
}

func Error() *zerolog.Event {
	return log.Error().Timestamp().Stack()
}

func Panic() *zerolog.Event {
	return log.Panic().Timestamp().Stack()
}

func Fatal() *zerolog.Event {
	return log.Fatal().Timestamp().Stack()
}

func With() zerolog.Context {
	return log.With().Stack()
}

type PrettyZerologWriter struct {
	wd                  string
	wasLastLogMultiline bool
}

type PrettyLogEntry struct {
	Timestamp  string
	Level      string
	Message    string
	Error      string
	StackTrace []interface{}

	OtherFields []PrettyField
}

type PrettyField struct {
	Name  string
	Value interface{}
}

var ColorFromLevel = map[string]string{
	"trace": Gray,
	"debug": Gray,
	"info":  BgBlue,
	"warn":  BgYellow,
	"error": BgRed,
	"fatal": BgRed,
	"panic": BgRed,
}

func NewPrettyZerologWriter() *PrettyZerologWriter {
	wd, _ := os.Getwd()
	return &PrettyZerologWriter{
		wd:                  wd,
		wasLastLogMultiline: false,
	}
}

func (w *PrettyZerologWriter) Write(p []byte) (int, error) {
	var fields map[string]interface{}
	err := json.Unmarshal(p, &fields)
	if err != nil {
		return os.Stderr.Write(p)
	}

	var pretty PrettyLogEntry
	for name, val := range fields {
		switch name {
		case zerolog.TimestampFieldName:
			t, err := time.Parse(time.RFC3339, val.(string))
			if err == nil {
				pretty.Timestamp = t.Format(time.DateTime)
			} else {
				pretty.Timestamp = val.(string)
			}
		case zerolog.LevelFieldName:
			pretty.Level = val.(string)
		case zerolog.MessageFieldName:
			pretty.Message = val.(string)
		case zerolog.ErrorFieldName:
			pretty.Error = val.(string)
		case zerolog.ErrorStackFieldName:
			pretty.StackTrace = val.([]interface{})
		default:
			pretty.OtherFields = append(pretty.OtherFields, PrettyField{
				Name:  name,
				Value: val,
			})
		}
	}

	sort.Slice(pretty.OtherFields, func(i, j int) bool {
		return strings.Compare(pretty.OtherFields[i].Name, pretty.OtherFields[j].Name) < 0
	})

	isMultiline := (pretty.Error != "" || pretty.StackTrace != nil || pretty.OtherFields != nil)

	var b strings.Builder
	if isMultiline || w.wasLastLogMultiline {
		b.WriteString("---------------------------------------\n")
	}
	b.WriteString(pretty.Timestamp)
	b.WriteString(" ")
	if pretty.Level != "" {
		b.WriteString(ColorFromLevel[pretty.Level])
		b.WriteString(Bold)
		b.WriteString(strings.ToUpper(pretty.Level))
		b.WriteString(Reset)
		b.WriteString(": ")
	}
	b.WriteString(pretty.Message)
	b.WriteString("\n")
	if pretty.Error != "" {
		b.WriteString("  " + Bold + Red + "ERROR:" + Reset + " ")
		b.WriteString(pretty.Error)
		b.WriteString("\n")
	}
	if len(pretty.OtherFields) > 0 {
		b.WriteString("  " + Bold + Blue + "Fields:" + Reset + "\n")
		for _, field := range pretty.OtherFields {
			valuePretty, _ := json.MarshalIndent(field.Value, "    ", "  ")
			b.WriteString("    ")
			b.WriteString(field.Name)
			b.WriteString(": ")
			b.WriteString(string(valuePretty))
			b.WriteString("\n")
		}
	}
	if pretty.StackTrace != nil {
		b.WriteString("  " + Bold + Blue + "Stack trace:" + Reset + "\n")
		for _, frame := range pretty.StackTrace {
			frameMap := frame.(map[string]interface{})
			file := frameMap["file"].(string)
			file = strings.Replace(file, w.wd, ".", 1)

			b.WriteString("    ")
			b.WriteString(frameMap["function"].(string))
			b.WriteString(" (")
			b.WriteString(file)
			b.WriteString(":")
			b.WriteString(strconv.Itoa(int(frameMap["line"].(float64))))
			b.WriteString(")\n")
		}
	}

	w.wasLastLogMultiline = isMultiline

	return os.Stderr.Write([]byte(b.String()))
}

func LogPanics(logger *zerolog.Logger) {
	if r := recover(); r != nil {
		LogPanicValue(logger, r, "recovered from panic")
	}
}

func LogPanicValue(logger *zerolog.Logger, val interface{}, msg string) {
	if logger == nil {
		logger = GlobalLogger()
	}

	if err, ok := val.(error); ok {
		logger.Error().Err(err).Msg(msg)
	} else {
		logger.Error().
			Interface("recovered", val).
			Interface(zerolog.ErrorStackFieldName, ee.Trace()).
			Msg(msg)
	}
}

const LoggerContextKey = "logger"

func AttachLoggerToContext(logger *zerolog.Logger, ctx context.Context) context.Context {
	return context.WithValue(ctx, LoggerContextKey, logger)
}

func ExtractLogger(ctx context.Context) *zerolog.Logger {
	ilogger := ctx.Value(LoggerContextKey)
	if ilogger == nil {
		return GlobalLogger()
	}
	return ilogger.(*zerolog.Logger)
}
