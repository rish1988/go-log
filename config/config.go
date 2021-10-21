package config

import "github.com/rish1988/go-log/colorful"

type LogOptions struct {
	ColorOptions
	Debug bool
}

type ColorOptions struct {
	Color bool
	Quiet bool
	TimeStampColorOptions
	Info colorful.Color
	Warn colorful.Color
	Debug colorful.Color
	Trace colorful.Color
	Fatal colorful.Color
	Error colorful.Color
}

type TimeStampColorOptions struct {
	TimeStamp bool
	colorful.Color
}
