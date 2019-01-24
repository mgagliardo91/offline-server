package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mgagliardo91/offline-common"
)

func setupRoutes(r *mux.Router) *mux.Router {
	r.HandleFunc("/event", eventHandler).Methods("POST")

	return r
}

func eventHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var offlineEvent common.OfflineEvent

	err := decoder.Decode(&offlineEvent)
	if err != nil {
		panic(err)
	}

	GetLogger().Tracef("Queueing event for processing: %+v", offlineEvent)
	GetExecutor().QueueTask(RawEvent, offlineEvent)
}
