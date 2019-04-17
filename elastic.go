package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	elastic "github.com/elastic/go-elasticsearch/v6"
	"github.com/mgagliardo91/go-utils"
)

type JsonObject map[string]interface{}

type Document interface {
	GetId() string
	GetTable() string
	SetCreateDate(date int64)
	SetModifiedDate(date int64)
}

type DocumentImpl struct {
	CreateDate   int64 `json:"createDate"`
	ModifiedDate int64 `json:"modifiedDate"`
}

func (doc *DocumentImpl) GetId() string {
	return ""
}

func (doc *DocumentImpl) GetTable() string {
	return ""
}

func (doc *DocumentImpl) SetCreateDate(date int64) {
	doc.CreateDate = date
}

func (doc *DocumentImpl) SetModifiedDate(date int64) {
	doc.ModifiedDate = date
}

var esClient *elastic.Client

func initClient() error {
	esURL := utils.GetEnvString("ELASTIC_SEARCH_URL", "")

	if esURL == "" {
		return errors.New("Invalid elastic search property")
	}

	esURLs := strings.Split(esURL, ",")

	cfg := elastic.Config{
		Addresses: esURLs,
	}

	es, err := elastic.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("Error creating the esClient: %s", err)
	}

	GetLogger().Infof("Connected to elastic search at: %s", esURLs)

	esClient = es
	return nil
}

func ensureIndices() error {
	GetLogger().Info("Ensuring indices")
	files, err := ioutil.ReadDir("./indices")

	if err != nil {
		return fmt.Errorf("Unable to read index directory. %s", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "json") {
			body, err := ioutil.ReadFile("./indices/" + file.Name())

			if err != nil {
				return fmt.Errorf("Unable to read index file: %s. %s", file.Name(), err)
			}

			var index JsonObject
			if err := bytesToJson(body, &index); err != nil {
				return err
			}

			if err := ensureIndex(strings.ReplaceAll(file.Name(), ".json", ""), index); err != nil {
				return err
			}
		}
	}

	return nil
}

func bytesToJson(value []byte, result *JsonObject) error {
	return readerToJson(bytes.NewReader(value), result)
}

func readerToJson(reader io.Reader, result *JsonObject) error {
	if err := json.NewDecoder(reader).Decode(result); err != nil {
		return fmt.Errorf("Unable to unmarshall value. %s", err)
	}

	return nil
}

func ensureIndex(name string, index map[string]interface{}) error {
	GetLogger().Infof("Ensuring index at alias: %s", name)

	res, err := esClient.Indices.ExistsAlias(
		[]string{name},
		esClient.Indices.ExistsAlias.WithContext(context.Background()),
	)

	if err != nil {
		return fmt.Errorf("Unable to check if alias exists for %s. %s", name, err)
	}

	if res.StatusCode == 200 {
		GetLogger().Infof("Alias exists: %s. Not creating", name)
		return nil
	}

	return createIndex(name, index)
}

func createIndex(name string, index map[string]interface{}) error {
	GetLogger().Infof("Creating index for alias %s", name)
	indexString, err := json.Marshal(index)
	indexName := fmt.Sprintf("%s_%d", name, time.Now().UnixNano()/int64(time.Millisecond))

	if err != nil {
		return fmt.Errorf("Unable to parse index with name %s. %s", name, err)
	}

	indexRes, err := esClient.Indices.Create(
		indexName,
		esClient.Indices.Create.WithContext(context.Background()),
		esClient.Indices.Create.WithBody(strings.NewReader(string(indexString))),
	)

	defer indexRes.Body.Close()

	var mapping JsonObject
	if err := readerToJson(indexRes.Body, &mapping); err != nil {
		return err
	}

	if err, hasError := mapping["error"]; hasError {
		return fmt.Errorf(
			"Unable to create index %s. type=%s reason=%s",
			indexName,
			err.(map[string]interface{})["type"],
			err.(map[string]interface{})["reason"],
		)
	}

	GetLogger().Infof("Created index for %s. Response: %+v", indexName, mapping)

	aliasRes, err := esClient.Indices.PutAlias(
		[]string{indexName},
		name,
		esClient.Indices.PutAlias.WithContext(context.Background()),
	)

	defer aliasRes.Body.Close()
	if err := readerToJson(aliasRes.Body, &mapping); err != nil {
		return err
	}

	if err, hasError := mapping["error"]; hasError {
		return fmt.Errorf(
			"Unable to create alias %s. type=%s reason=%s",
			name,
			err.(map[string]interface{})["type"],
			err.(map[string]interface{})["reason"],
		)
	}

	GetLogger().Infof("Created alias for %s. Response: %+v", name, mapping)

	return nil
}

func indexDocument(doc Document) error {
	id := doc.GetId()

	if len(id) == 0 {
		return fmt.Errorf("Invalid document ID: %+v", doc)
	}

	table := doc.GetTable()

	if len(table) == 0 {
		return fmt.Errorf("Invalid table: %+v", doc)
	}

	currentTime := time.Now().UnixNano() / int64(time.Millisecond)

	doc.SetCreateDate(currentTime)
	doc.SetModifiedDate(currentTime)

	fetchRes, err := esClient.Get(
		table,
		id,
		esClient.Get.WithContext(context.Background()),
		esClient.Get.WithSourceInclude("createDate"),
	)

	if err != nil {
		return fmt.Errorf("Unable to fetch document: %+v. Error: %s", doc, err)
	}

	defer fetchRes.Body.Close()
	var current JsonObject
	if err := readerToJson(fetchRes.Body, &current); err != nil {
		return err
	}

	if source, hasSource := current["_source"]; hasSource {
		b, err := json.Marshal(source.(map[string]interface{}))

		if err != nil {
			return fmt.Errorf("Unable to marshal existing document %+v. %s", current, err)
		}

		var existing DocumentImpl
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&existing); err == nil {
			doc.SetCreateDate(existing.CreateDate)
		}
	}

	b, err := json.Marshal(doc)

	if err != nil {
		return fmt.Errorf("Unable to marshal document. %s", err)
	}

	res, err := esClient.Index(
		table,
		strings.NewReader(string(b)),
		esClient.Index.WithContext(context.Background()),
		esClient.Index.WithDocumentID(id),
	)

	if err != nil {
		return fmt.Errorf("Unable to index document: %+v. Error: %s", doc, err)
	}

	defer res.Body.Close()
	var response JsonObject
	if err := readerToJson(res.Body, &response); err != nil {
		return err
	}

	if err, hasError := response["error"]; hasError {
		return fmt.Errorf(
			"Unable to index document %+v. type=%s reason=%s",
			doc,
			err.(map[string]interface{})["type"],
			err.(map[string]interface{})["reason"],
		)
	}

	GetLogger().Debugf("Indexed document %+v. Response: %+v", doc, response)

	return nil
}
