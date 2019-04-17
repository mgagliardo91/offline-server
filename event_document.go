package main

import (
	common "github.com/mgagliardo91/offline-common"
)

type OfflineEventDocument struct {
	DocumentImpl
	common.OfflineEvent
}

func (e OfflineEventDocument) GetId() string {
	return e.ID
}

func (e OfflineEventDocument) GetTable() string {
	return "event"
}
