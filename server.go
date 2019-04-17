package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/go-utils"

	"github.com/gorilla/mux"
)

const (
	RawEvent blacksmith.TaskName = iota
)

var (
	maxWorkers      = utils.GetEnvInt("MAX_WORKERS", 10)
	gracefulTimeout = utils.GetEnvDuration("SERVER_GRACEFUL_TIMEOUT", 1*time.Minute)
)

var eventExecutor *blacksmith.Blacksmith
var logger *utils.LogWrapper

func main() {
	initClient()
	if err := ensureIndices(); err != nil {
		logger.Errorf("Unable to ensure indices: %s", err)
	}
	utils.SetLoggerLevel(blacksmith.LoggerName, "info")
	router := createRouter()
	server := &http.Server{
		Handler:      router,
		Addr:         "localhost:3000",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
	GetExecutor().Run()

	GetLogger().Infof("Server starting at %s", server.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			GetLogger().Errorln(err)
			return
		}

	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()

	GetExecutor().Stop()

	server.Shutdown(ctx)
	GetLogger().Info("Shutting down...")
	os.Exit(0)
}

func createRouter() *mux.Router {
	r := mux.NewRouter()
	return setupRoutes(r)
}

func GetExecutor() *blacksmith.Blacksmith {
	if eventExecutor == nil {
		eventExecutor = blacksmith.New(maxWorkers)
		eventExecutor.SetHandler(RawEvent, processRawEvent)
	}

	return eventExecutor
}

func GetLogger() *utils.LogWrapper {
	if logger == nil {
		logger = utils.NewLogger("Server")
	}

	return logger
}
