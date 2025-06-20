package log

import (
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
)

// InitLog 初始化日志
func InitLog() {
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.DateTime,
		FullTimestamp:   true,
		CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	})
	logrus.Infof("log init finish.......")
}
