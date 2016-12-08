/*
 * Copyright (c) 2016 TFG Co <backend@tfgco.com>
 * Author: TFG Co <backend@tfgco.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package worker

import (
	"encoding/json"
	"fmt"
	"time"

	workers "github.com/jrallison/go-workers"
	"github.com/satori/go.uuid"
	"github.com/spf13/viper"
	"github.com/topfreegames/marathon/extensions"
	"github.com/topfreegames/marathon/model"
	"github.com/uber-go/zap"
)

// ProcessBatchWorker is the ProcessBatchWorker struct
type ProcessBatchWorker struct {
	Config     *viper.Viper
	Kafka      *extensions.KafkaClient
	Logger     zap.Logger
	MarathonDB *extensions.PGClient
	Zookeeper  *extensions.ZookeeperClient
}

// NewProcessBatchWorker gets a new ProcessBatchWorker
func NewProcessBatchWorker(config *viper.Viper, logger zap.Logger) *ProcessBatchWorker {
	zookeeper, err := extensions.NewZookeeperClient(config, logger)
	checkErr(err)
	//Wait 10s at max for a connection
	zookeeper.WaitForConnection(10)
	kafka, err := extensions.NewKafkaClient(zookeeper, config, logger)
	checkErr(err)
	marathonDB, err := extensions.NewPGClient("db", config, logger)
	checkErr(err)
	batchWorker := &ProcessBatchWorker{
		Config:     config,
		Logger:     logger,
		Kafka:      kafka,
		Zookeeper:  zookeeper,
		MarathonDB: marathonDB,
	}
	return batchWorker
}

func (batchWorker *ProcessBatchWorker) sendToKafka(service, topic string, msg, metadata map[string]interface{}, deviceToken string, expiresAt int64) error {
	pushExpiry := expiresAt / 1000000000 // convert from nanoseconds to seconds
	switch service {
	case "apns":
		_, _, err := batchWorker.Kafka.SendAPNSPush(topic, deviceToken, msg, metadata, pushExpiry)
		if err != nil {
			return err
		}
	case "gcm":
		_, _, err := batchWorker.Kafka.SendGCMPush(topic, deviceToken, msg, metadata, pushExpiry)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("service should be in ['apns', 'gcm']")
	}
	return nil
}

func (batchWorker *ProcessBatchWorker) getJobTemplatesByLocale(jobID uuid.UUID) (map[string]*model.Template, error) {
	templateByLocale := make(map[string]*model.Template)
	job := model.Job{
		ID: jobID,
	}
	err := batchWorker.MarathonDB.DB.Select(&job)
	if err != nil {
		return nil, err
	}
	var templates []model.Template
	template := &model.Template{
		Name:  job.TemplateName,
		AppID: job.AppID,
	}
	err = batchWorker.MarathonDB.DB.Model(template).Select(&templates)
	if err != nil {
		return nil, err
	}
	for _, tpl := range templates {
		templateByLocale[tpl.Locale] = &tpl
	}

	return templateByLocale, nil
}

func (batchWorker *ProcessBatchWorker) updateJobBatchesInfo(jobID uuid.UUID) error {
	job := model.Job{}
	_, err := batchWorker.MarathonDB.DB.Model(&job).Set("completed_batches = completed_batches + 1").Where("id = ?", jobID).Returning("*").Update()
	if err != nil {
		return err
	}
	if job.CompletedBatches >= job.TotalBatches && job.CompletedAt == 0 {
		job.CompletedAt = time.Now().UnixNano()
		_, err = batchWorker.MarathonDB.DB.Model(&job).Column("completed_at").Update()
	}
	return err
}

// Process processes the messages sent to batch worker queue and send them to kafka
func (batchWorker *ProcessBatchWorker) Process(message *workers.Msg) {
	// l := workers.Logger
	arr, err := message.Args().Array()
	checkErr(err)

	parsed, err := ParseProcessBatchWorkerMessageArray(arr)
	checkErr(err)

	templatesByLocale, err := batchWorker.getJobTemplatesByLocale(parsed.JobID)
	checkErr(err)

	topicTemplate := batchWorker.Config.GetString("workers.topicTemplate")
	topic := BuildTopicName(parsed.AppName, parsed.Service, topicTemplate)
	for _, user := range parsed.Users {
		var template *model.Template
		if val, ok := templatesByLocale[user.Locale]; ok {
			template = val
		} else {
			template = templatesByLocale["en"]
		}

		if template == nil {
			checkErr(fmt.Errorf("there is no template for the given locale or 'en'"))
		}

		msgStr := BuildMessageFromTemplate(template, parsed.Context)
		var msg map[string]interface{}
		err = json.Unmarshal([]byte(msgStr), &msg)
		checkErr(err)
		err = batchWorker.sendToKafka(parsed.Service, topic, msg, parsed.Metadata, user.Token, parsed.ExpiresAt)
		checkErr(err)
	}

	err = batchWorker.updateJobBatchesInfo(parsed.JobID)
	checkErr(err)
}
