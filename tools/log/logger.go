package log

import "github.com/sirupsen/logrus"

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
