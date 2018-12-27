package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mgagliardo91/offline-common"
)

func setupRoutes(r *mux.Router) *mux.Router {
	r.HandleFunc("/event", EventHandler).Methods("POST")

	return r
}

func EventHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var offlineEvent common.OfflineEvent

	err := decoder.Decode(&offlineEvent)
	if err != nil {
		panic(err)
	}
	log.Printf("Recieved date: %v", offlineEvent.Date)
}
