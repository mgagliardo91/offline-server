package main

import (
	"encoding/json"

	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/offline-common"
)

func processRawEvent(task blacksmith.Task) {
	rawEvent, ok := task.Payload.(common.OfflineEvent)
	if !ok {
		GetLogger().Errorf("Unable to extract OfflineEvent from task %+v", task.Payload)
		return
	}

	GetLogger().Infof("Processing event: %s", rawEvent.Title)

	jsonValue, _ := json.Marshal(rawEvent)
	GetLogger().Infof("Json Value: %s", jsonValue)
}
