package app

import (
	"subpun/internal/config"
	"subpun/subpub"

	"github.com/sirupsen/logrus"
)

// App has main components
type App struct {
	Logger *logrus.Logger
	SubPub subpub.SubPub
}

// NewApp
func NewApp(cfg *config.Config) *App {
	// Create logger
	logger := logrus.New()
	//Error checked in config initialize
	level, _ := logrus.ParseLevel(cfg.Logging.Level)
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006/01/02 15:04:05",
	})

	//Create subPub
	sp := subpub.NewSubPub()

	return &App{
		Logger: logger,
		SubPub: sp,
	}
}
