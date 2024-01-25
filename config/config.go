package config

import (
	"github.com/rish1988/go-log/colorful"
)

type LogOptions struct {
	ColorOptions
	*FileOptions
	Debug bool
}

type ColorOptions struct {
	Color bool
	Quiet bool
	TimeStampColorOptions
	Info  colorful.Color
	Warn  colorful.Color
	Debug colorful.Color
	Trace colorful.Color
	Fatal colorful.Color
	Error colorful.Color
}

type FileOptions struct {
	TimeZone   string
	FileName   string
	DateFormat string
	LogsDir    string
	*RotationPolicyOptions
}

type RotationPolicyOptions struct {
	// Must be a valid cron expression
	RotationInterval string
	MaxFiles         int
}

type TimeStampColorOptions struct {
	TimeStamp bool
	colorful.Color
}
