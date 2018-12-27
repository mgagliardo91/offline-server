package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/go-utils"

	"github.com/gorilla/mux"
)

var (
	maxWorkers      = utils.GetEnvInt("MAX_WORKERS", 10)
	gracefulTimeout = utils.GetEnvDuration("SERVER_GRACEFUL_TIMEOUT", 1*time.Minute)
)

var eventExecutor blacksmith.Blacksmith

func main() {
	router := createRouter()
	server := &http.Server{
		Handler:      router,
		Addr:         "localhost:3000",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	eventExecutor := blacksmith.New(maxWorkers)
	eventExecutor.Run()

	log.Println("Starting the server...")
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer cancel()

	eventExecutor.Stop()

	server.Shutdown(ctx)
	log.Println("Shutting down...")
	os.Exit(0)
}

func createRouter() *mux.Router {
	r := mux.NewRouter()
	return setupRoutes(r)
}
