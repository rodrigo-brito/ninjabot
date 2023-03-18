package log

import "github.com/sirupsen/logrus"

var (
	WarnLevel  = logrus.WarnLevel
	InfoLevel  = logrus.InfoLevel
	DebugLevel = logrus.DebugLevel
	ErrorLevel = logrus.ErrorLevel
	FatalLevel = logrus.FatalLevel
	PanicLevel = logrus.PanicLevel
)

type (
	TextFormatter = logrus.TextFormatter
	Level         = logrus.Level
)

func CheckErr(level logrus.Level, err error) {
	if err != nil {
		Log(level, err)
	}
}

func Log(level logrus.Level, messages ...interface{}) {
	switch level {
	case logrus.InfoLevel:
		logrus.Info(messages...)
	case logrus.WarnLevel:
		logrus.Warn(messages...)
	case logrus.ErrorLevel:
		logrus.Error(messages...)
	case logrus.FatalLevel:
		logrus.Fatal(messages...)
	case logrus.PanicLevel:
		logrus.Panic(messages...)
	case logrus.DebugLevel:
		fallthrough
	default:
		logrus.Debug(messages...)
	}
}

func SetFormatter(formatter logrus.Formatter) {
	logrus.SetFormatter(formatter)
}

func SetLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return logrus.WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return logrus.WithFields(fields)
}

func Info(messages ...interface{}) {
	logrus.Info(messages...)
}

func Infof(format string, messages ...interface{}) {
	logrus.Infof(format, messages...)
}

func Warn(messages ...interface{}) {
	logrus.Warn(messages...)
}

func Warnf(format string, messages ...interface{}) {
	logrus.Warnf(format, messages...)
}

func Error(messages ...interface{}) {
	logrus.Error(messages...)
}

func Errorf(format string, messages ...interface{}) {
	logrus.Errorf(format, messages...)
}

func Fatal(messages ...interface{}) {
	logrus.Fatal(messages...)
}

func Debug(messages ...interface{}) {
	logrus.Debug(messages...)
}

func Debugf(format string, messages ...interface{}) {
	logrus.Debugf(format, messages...)
}
