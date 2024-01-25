// The colorful and simple logging library
// Copyright (c) 2017 Fadhli Dzil Ikram

package log

import (
	"fmt"
	"github.com/rish1988/go-log/colorful"
	"github.com/rish1988/go-log/config"
	"github.com/rish1988/go-log/cronjob"
	"github.com/rish1988/go-log/files"
	"github.com/robfig/cron"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/crypto/ssh/terminal"
)

// FdWriter interface extends existing io.Writer with file descriptor function
// support
type FdWriter interface {
	io.Writer
	Fd() uintptr
}

type FdWriters []FdWriter

func NewFdWriters(files ...FdWriter) FdWriters {
	return files
}

func (f *FdWriters) Write(t []byte, p []byte) (n int, err error) {
	for _, writer := range *f {
		isTerminal := terminal.IsTerminal(int(writer.Fd()))

		if isTerminal {
			if n, err = writer.Write(t); err != nil {
				return n, err
			}
		} else if n, err = writer.Write(p); err != nil {
			return n, err

		}
	}
	return len(t), nil
}

// Logger struct define the underlying storage for single logger
type Logger struct {
	mu            sync.RWMutex
	color         bool
	out           FdWriters
	debug         bool
	timestamp     bool
	quiet         bool
	colorSettings config.ColorOptions
	colorBuf      colorful.ColorBuffer
	noColorBuf    colorful.ColorBuffer
	cron          *cron.Cron
	logFile       *os.File
	timeFormat    string
	timeZone      *time.Location
}

// Prefix struct define plain and color byte
type Prefix struct {
	Plain []byte
	Color []byte
	File  bool
}

var (
	// Plain prefix template
	plainFatal = []byte("[FATAL] ")
	plainError = []byte("[ERROR] ")
	plainWarn  = []byte("[WARN]  ")
	plainInfo  = []byte("[INFO]  ")
	plainDebug = []byte("[DEBUG] ")
	plainTrace = []byte("[TRACE] ")

	// FatalPrefix show fatal prefix
	FatalPrefix = Prefix{
		Plain: plainFatal,
		Color: colorful.Red(plainFatal),
		File:  true,
	}

	// ErrorPrefix show error prefix
	ErrorPrefix = Prefix{
		Plain: plainError,
		Color: colorful.Red(plainError),
		File:  true,
	}

	// WarnPrefix show warn prefix
	WarnPrefix = Prefix{
		Plain: plainWarn,
		Color: colorful.Orange(plainWarn),
	}

	// InfoPrefix show info prefix
	InfoPrefix = Prefix{
		Plain: plainInfo,
		Color: colorful.Green(plainInfo),
	}

	// DebugPrefix show info prefix
	DebugPrefix = Prefix{
		Plain: plainDebug,
		Color: colorful.Purple(plainDebug),
		File:  true,
	}

	// TracePrefix show info prefix
	TracePrefix = Prefix{
		Plain: plainTrace,
		Color: colorful.Cyan(plainTrace),
	}
)

type Message struct {
	Plain []byte
	Color []byte
}

type MessageType int

const (
	Fatal = iota
	Error
	Warn
	Info
	Debug
	Trace
)

func (l *Logger) coloredMessage(messageType MessageType, data string) Message {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = data + "\n"
	}

	message := Message{
		Plain: []byte(data),
	}

	switch messageType {
	case Fatal:
		fatalColor := l.colorSettings.Fatal
		if fatalColor != nil {
			message.Color = fatalColor(message.Plain)
			FatalPrefix.Color = fatalColor(FatalPrefix.Plain)
		} else {
			message.Color = colorful.Red(message.Plain)
		}
	case Error:
		errorColor := l.colorSettings.Error
		if errorColor != nil {
			message.Color = errorColor(message.Plain)
			ErrorPrefix.Color = errorColor(ErrorPrefix.Plain)
		} else {
			message.Color = colorful.Red(message.Plain)
		}
	case Warn:
		warnColor := l.colorSettings.Warn
		if warnColor != nil {
			message.Color = warnColor(message.Plain)
			WarnPrefix.Color = warnColor(WarnPrefix.Plain)
		} else {
			message.Color = colorful.Orange(message.Plain)
		}
	case Info:
		infoColor := l.colorSettings.Info
		if infoColor != nil {
			message.Color = infoColor(message.Plain)
			InfoPrefix.Color = infoColor(InfoPrefix.Plain)
		} else {
			message.Color = colorful.Green(message.Plain)
		}
	case Debug:
		debugColor := l.colorSettings.Debug
		if debugColor != nil {
			message.Color = debugColor(message.Plain)
			DebugPrefix.Color = debugColor(DebugPrefix.Plain)
		} else {
			message.Color = colorful.Purple(message.Plain)
		}
	case Trace:
		traceColor := l.colorSettings.Trace
		if traceColor != nil {
			message.Color = traceColor(message.Plain)
			TracePrefix.Color = traceColor(TracePrefix.Plain)
		} else {
			message.Color = colorful.Cyan(message.Plain)
		}
	}
	return message
}

// New returns new Logger instance with predefined writer output and
// automatically detect terminal coloring support
func New(out FdWriters, options config.LogOptions) *Logger {
	if options.FileOptions != nil && files.DirExists(options.LogsDir) {
		log := getLogger(options)
		c := cronjob.NewCron(options.TimeZone)

		var (
			cronInterval string
			maxFileCount int
		)

		if options.RotationPolicyOptions != nil {
			cronInterval = options.RotationInterval
			maxFileCount = options.MaxFiles
		} else {
			cronInterval = "@midnight"
			maxFileCount = math.MaxInt64
		}

		if err := c.AddJob(cronInterval, cron.FuncJob(func() {
			log = getLogger(options)
			time.Sleep(time.Second)
		})); err != nil {
			fmt.Printf("Failed to add logger cronjob. Reason: %s\n", err)
			return log
		}

		if err := c.AddJob(cronInterval, cron.FuncJob(func() {
			cleanupOldLogs(options.LogsDir, maxFileCount)
		})); err != nil {
			fmt.Printf("Failed to add logger cronjob to remove old log files. Reason: %s\n", err)
			return log
		}
		c.Start()

		go func() {
			for {
				select {
				case <-options.Context.Done():
					fmt.Printf("Stopping logger.\n")
					c.Stop()
					return
				}
			}
		}()

		return log
	}

	return &Logger{
		color:         isTerminal(out),
		out:           out,
		timestamp:     options.TimeStamp,
		debug:         options.Debug,
		quiet:         options.Quiet,
		colorSettings: options.ColorOptions,
		timeZone:      time.Now().Location(),
		timeFormat:    "02-Jan-2006",
	}
}

func isTerminal(out FdWriters) bool {
	for _, o := range out {
		if terminal.IsTerminal(int(o.Fd())) {
			return true
		}
	}
	return false
}

func getLogger(opts config.LogOptions) *Logger {
	var (
		writers  FdWriters
		location *time.Location
		err      error
	)

	file := logFile(opts.LogsDir, opts.FileName, opts.DateFormat)
	timeZone := opts.TimeZone
	dateFormat := opts.DateFormat

	if len(timeZone) == 0 {
		timeZone = "Local"
	}

	if location, err = time.LoadLocation(timeZone); err != nil {
		fmt.Printf("Invalid timezone %s", timeZone)
	}

	if len(dateFormat) == 0 {
		dateFormat = "02-Jan-2006"
	}

	if file != nil {
		writers = NewFdWriters(os.Stderr, file)
	} else {
		writers = NewFdWriters(os.Stderr)
	}

	return &Logger{
		color:         isTerminal(writers),
		out:           writers,
		timestamp:     opts.TimeStamp,
		debug:         opts.Debug,
		quiet:         opts.Quiet,
		colorSettings: opts.ColorOptions,
		logFile:       file,
		timeFormat:    dateFormat,
		timeZone:      location,
	}
}

func cleanupOldLogs(logsDir string, maxDays int) {
	if rxlogs, err := files.Remove(logsDir, maxDays); err != nil {
		fmt.Printf("%v\n", err)
	} else {
		for _, rfile := range rxlogs {
			fmt.Printf("Removing old log file [ %s ] \n", rfile)
		}
	}
}

func logFile(logsDir, fileName, dateFormat string) *os.File {
	t := time.Now()
	date := t.Format(dateFormat)
	if len(logsDir) != 0 {
		logFileName := fmt.Sprintf("%s/%s-%s.log", logsDir, fileName, date)
		if file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err != nil {
			return nil
		} else {
			return file
		}
	} else {
		return nil
	}
}

func (l *Logger) GetLogFile() *os.File {
	return l.logFile
}

// IsDebug check the state of debugging output
func (l *Logger) IsDebug() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.debug
}

// IsQuiet check for quiet state
func (l *Logger) IsQuiet() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.quiet
}

// Output print the actual value
func (l *Logger) Output(depth int, prefix Prefix, data Message) error {
	// Check if quiet is requested, and try to return no error and be quiet
	if l.IsQuiet() {
		return nil
	}
	// Get current time
	now := time.Now().In(l.timeZone)
	// Temporary storage for file and line tracing
	var file string
	var line int
	var fn string
	// Check if the specified prefix needs to be included with file logging
	if prefix.File {
		var ok bool
		var pc uintptr

		// Get the caller filename and line
		if pc, file, line, ok = runtime.Caller(depth + 1); !ok {
			file = "<unknown file>"
			fn = "<unknown function>"
			line = 0
		} else {
			file = filepath.Base(file)
			fn = runtime.FuncForPC(pc).Name()
		}
	}
	// Acquire exclusive access to the shared buffer
	l.mu.Lock()
	defer l.mu.Unlock()
	// Reset buffer so it start from the begining
	l.colorBuf.Reset()
	l.noColorBuf.Reset()
	// Write prefix to the buffer

	l.noColorBuf.Append(prefix.Plain)
	if l.color {
		l.colorBuf.Append(prefix.Color)
	} else {
		l.colorBuf.Append(prefix.Plain)
	}
	// Check if the log require timestamping
	if l.timestamp {
		// Print timestamp color if color enabled
		if l.color {
			l.colorBuf.Blue()
		}
		// Print date and time

		l.colorBuf.Append([]byte(now.Format(l.timeFormat)))
		l.noColorBuf.Append([]byte(now.Format(l.timeFormat)))

		l.colorBuf.AppendByte(' ')
		l.noColorBuf.AppendByte(' ')

		hour, minutes, sec := now.Clock()

		l.colorBuf.AppendInt(hour, 2)
		l.noColorBuf.AppendInt(hour, 2)

		l.colorBuf.AppendByte(':')
		l.noColorBuf.AppendByte(':')

		l.colorBuf.AppendInt(minutes, 2)
		l.noColorBuf.AppendInt(minutes, 2)

		l.colorBuf.AppendByte(':')
		l.noColorBuf.AppendByte(':')

		l.colorBuf.AppendInt(sec, 2)
		l.noColorBuf.AppendInt(sec, 2)

		l.colorBuf.AppendByte(' ')
		l.noColorBuf.AppendByte(' ')

		// Print reset color if color enabled
		if l.color {
			l.colorBuf.Off()
		}
	}
	// Add caller filename and line if enabled
	if prefix.File {
		// Print color start if enabled
		if l.color {
			l.colorBuf.Orange()
		}
		// Print filename and line
		l.colorBuf.Append([]byte(fn))
		l.noColorBuf.Append([]byte(fn))

		l.colorBuf.AppendByte(':')
		l.noColorBuf.AppendByte(':')

		l.colorBuf.Append([]byte(file))
		l.noColorBuf.Append([]byte(file))

		l.colorBuf.AppendByte(':')
		l.noColorBuf.AppendByte(':')

		l.colorBuf.AppendInt(line, 0)
		l.noColorBuf.AppendInt(line, 0)

		l.colorBuf.AppendByte(' ')
		l.noColorBuf.AppendByte(' ')

		// Print color stop
		if l.color {
			l.colorBuf.Off()
		}
	}

	l.noColorBuf.Append(data.Plain)
	// Print the actual string data from caller
	if l.color {
		l.colorBuf.Append(data.Color)
	} else {
		l.colorBuf.Append(data.Plain)
	}

	// Flush buffer to output
	_, err := l.out.Write(l.colorBuf.Buffer, l.noColorBuf.Buffer)
	return err
}

// Fatal print fatal coloredMessage to output and quit the application with status 1
func (l *Logger) Fatal(v ...interface{}) {
	l.Output(1, FatalPrefix, l.coloredMessage(Fatal, fmt.Sprintln(v...)))
	os.Exit(1)
}

// Fatalf print formatted fatal coloredMessage to output and quit the application
// with status 1
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Output(1, FatalPrefix, l.coloredMessage(Fatal, fmt.Sprintf(format, v...)))
	os.Exit(1)
}

// Error print error coloredMessage to output
func (l *Logger) Error(v ...interface{}) {
	l.Output(1, ErrorPrefix, l.coloredMessage(Error, fmt.Sprintln(v...)))
}

// Errorf print formatted error coloredMessage to output
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Output(1, ErrorPrefix, l.coloredMessage(Error, fmt.Sprintf(format, v...)))
}

// Warn print warning coloredMessage to output
func (l *Logger) Warn(v ...interface{}) {
	l.Output(1, WarnPrefix, l.coloredMessage(Warn, fmt.Sprintln(v...)))
}

// Warnf print formatted warning coloredMessage to output
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Output(1, WarnPrefix, l.coloredMessage(Warn, fmt.Sprintf(format, v...)))
}

// Info print informational coloredMessage to output
func (l *Logger) Info(v ...interface{}) {
	l.Output(1, InfoPrefix, l.coloredMessage(Info, fmt.Sprintln(v...)))
}

// Infof print formatted informational coloredMessage to output
func (l *Logger) Infof(format string, v ...interface{}) {
	l.Output(1, InfoPrefix, l.coloredMessage(Info, fmt.Sprintf(format, v...)))
}

// Debug print debug coloredMessage to output if debug output enabled
func (l *Logger) Debug(v ...interface{}) {
	if l.IsDebug() {
		l.Output(1, DebugPrefix, l.coloredMessage(Debug, fmt.Sprintln(v...)))
	}
}

// Debugf print formatted debug coloredMessage to output if debug output enabled
func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.IsDebug() {
		l.Output(1, DebugPrefix, l.coloredMessage(Debug, fmt.Sprintf(format, v...)))
	}
}

// Trace print trace coloredMessage to output if debug output enabled
func (l *Logger) Trace(v ...interface{}) {
	if l.IsDebug() {
		l.Output(1, TracePrefix, l.coloredMessage(Trace, fmt.Sprintln(v...)))
	}
}

// Tracef print formatted trace coloredMessage to output if debug output enabled
func (l *Logger) Tracef(format string, v ...interface{}) {
	if l.IsDebug() {
		l.Output(1, TracePrefix, l.coloredMessage(Trace, fmt.Sprintf(format, v...)))
	}
}
