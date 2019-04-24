/*
 * Copyright (c) 2019 TFG Co <backend@tfgco.com>
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

/* this worker will not create an csv or a control group */

package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pg "gopkg.in/pg.v5"

	workers "github.com/jrallison/go-workers"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/log"
	"github.com/topfreegames/marathon/model"
	"github.com/uber-go/zap"
)

// DirectPartMsg saves information about a block to process
type DirectPartMsg struct {
	SmallestSeqID uint64 // not in the interval
	BiggestSeqID  uint64 // in the interval
	JobUUID       uuid.UUID
}

const nameDirectWorker = "direct_worker"

// DirectWorker is the DirectWorker struct
type DirectWorker struct {
	Logger  zap.Logger
	Workers *Worker
}

// NewDirectWorker gets a new DirectWorker
func NewDirectWorker(workers *Worker) *DirectWorker {
	b := &DirectWorker{
		Logger:  workers.Logger.With(zap.String("worker", "DirectWorker")),
		Workers: workers,
	}
	b.Logger.Debug("Configured DirectWorker successfully")
	return b
}

func (b *DirectWorker) sendToKafka(service, topic string, msg, messageMetadata map[string]interface{}, pushMetadata map[string]interface{}, deviceToken string, expiresAt int64, templateName string) error {
	pushExpiry := expiresAt / 1000000000 // convert from nanoseconds to seconds
	switch service {
	case "apns":
		err := b.Workers.Kafka.SendAPNSPush(topic, deviceToken, msg, messageMetadata, pushMetadata, pushExpiry, templateName)
		if err != nil {
			return err
		}
	case "gcm":
		err := b.Workers.Kafka.SendGCMPush(topic, deviceToken, msg, messageMetadata, pushMetadata, pushExpiry, templateName)
		if err != nil {
			return err
		}
	default:
		panic("service should be in ['apns', 'gcm']")
	}
	return nil
}

func (b *DirectWorker) getJob(jobID uuid.UUID) (*model.Job, error) {
	job := model.Job{
		ID: jobID,
	}
	err := b.Workers.MarathonDB.DB.Model(&job).Column("job.*", "App").Where("job.id = ?", job.ID).Select()
	return &job, err
}

func (b *DirectWorker) addCompletedTokens(job *model.Job, nTokens int) error {
	_, err := b.Workers.MarathonDB.DB.Model(&job).Set("completed_tokens = completed_tokens + ?", nTokens).Where("id = ?", job.ID).Update()
	return err
}

func (b *DirectWorker) addCompletedBatch(job *model.Job) error {
	_, err := b.Workers.MarathonDB.DB.Model(&job).Set("completed_batches = completed_batches + 1").Where("id = ?", job.ID).Update()
	return err
}

func (b *DirectWorker) checkComplete(job *model.Job) (bool, error) {
	err := b.Workers.MarathonDB.DB.Model(&job).Where("id = ?", job.ID).Select()
	return job.CompletedBatches == job.TotalBatches, err
}

func (b *DirectWorker) getJobTemplatesByNameAndLocale(appID uuid.UUID, templateName string) (map[string]map[string]model.Template, error) {
	var templates []model.Template
	var err error
	if len(strings.Split(templateName, ",")) > 1 {
		err = b.Workers.MarathonDB.DB.Model(&templates).Where(
			"app_id = ? AND name IN (?)",
			appID,
			pg.In(strings.Split(templateName, ",")),
		).Select()
	} else {
		err = b.Workers.MarathonDB.DB.Model(&templates).Where(
			"app_id = ? AND name = ?",
			appID,
			templateName,
		).Select()
	}
	if err != nil {
		return nil, err
	}
	templateByLocale := make(map[string]map[string]model.Template)
	for _, tpl := range templates {
		if templateByLocale[tpl.Name] != nil {
			templateByLocale[tpl.Name][tpl.Locale] = tpl
		} else {
			templateByLocale[tpl.Name] = map[string]model.Template{
				tpl.Locale: tpl,
			}
		}
	}

	if len(templateByLocale) == 0 {
		return nil, fmt.Errorf("No templates were found with name %s", templateName)
	}
	return templateByLocale, nil
}

func (b *DirectWorker) getQuery(job *model.Job, msg DirectPartMsg) string {
	filters := job.Filters
	whereClause := GetWhereClauseFromFilters(filters)
	query := fmt.Sprintf("SELECT user_id, token, locale, tz FROM %s", GetPushDBTableName(job.App.Name, job.Service))
	query = fmt.Sprintf("%s WHERE seq_id > %d AND seq_id <= %d", query, msg.SmallestSeqID, msg.BiggestSeqID)
	if (whereClause) != "" {
		query = fmt.Sprintf("%s AND %s;", query, whereClause)
	}
	return query
}

// Process processes the messages sent to batch worker queue and send them to kafka
func (b *DirectWorker) Process(message *workers.Msg) {
	l := b.Logger.With(
		zap.String("worker", nameDirectWorker),
	)
	log.I(l, "starting")

	var msg DirectPartMsg
	data := message.Args().ToJson()
	err := json.Unmarshal([]byte(data), &msg)
	checkErr(l, err)

	job, err := b.getJob(msg.JobUUID)
	checkErr(l, err)
	b.Workers.Statsd.Incr("starting_direct_part", job.Labels(), 1)

	if job.ExpiresAt > 0 && job.ExpiresAt < time.Now().UnixNano() {
		log.I(l, "expired")
		return
	}

	switch job.Status {
	case "circuitbreak":
		log.I(l, "circuit break")
		return
	case "paused":
		log.I(l, "paused")
		return
	case "stopped":
		log.I(l, "stopped")
		return
	default:
		log.D(l, "valid")
	}

	templatesByNameAndLocale, err := b.getJobTemplatesByNameAndLocale(job.AppID, job.TemplateName)
	b.checkErr(job, err)

	topicTemplate := b.Workers.Config.GetString("workers.topicTemplate")
	topic := BuildTopicName(job.App.Name, job.Service, topicTemplate)

	var users []User
	start := time.Now()
	_, err = b.Workers.PushDB.DB.Query(&users, b.getQuery(job, msg))
	b.Workers.Statsd.Timing("get_from_pg", time.Now().Sub(start), job.Labels(), 1)

	successfulUsers := len(users)
	for _, user := range users {
		templateName := job.TemplateName
		templateNames := strings.Split(job.TemplateName, ",")

		if templateNames != nil && len(templateNames) > 1 {
			templateName = RandomElementFromSlice(templateNames)
			log.D(l, "selected template", func(cm log.CM) {
				cm.Write(zap.Object("name", templateName))
			})
		}

		templatesByLocale := templatesByNameAndLocale[templateName]
		var template model.Template
		if val, ok := templatesByLocale[strings.ToLower(user.Locale)]; ok {
			template = val
		} else if val, ok := templatesByLocale["en"]; ok {
			template = val
		} else {
			b.checkErr(job, fmt.Errorf("there is no template for the given locale or 'en'"))
		}

		msgStr, msgErr := BuildMessageFromTemplate(template, job.Context)
		b.checkErr(job, msgErr)

		var msg map[string]interface{}
		err = json.Unmarshal([]byte(msgStr), &msg)

		b.checkErr(job, err)
		pushMetadata := map[string]interface{}{
			"userId":       user.UserID,
			"pushTime":     time.Now().Unix(),
			"templateName": templateName,
			"jobId":        job.ID.String(),
			"pushType":     "massive",
			"muid":         uuid.NewV4().String(),
		}

		dryRun := false
		if val, ok := job.Metadata["dryRun"]; ok {
			if dryRun, ok = val.(bool); ok {
				pushMetadata["dryRun"] = dryRun
			}
		}

		err = b.sendToKafka(job.Service, topic, msg, job.Metadata, pushMetadata, user.Token, job.ExpiresAt, templateName)
		if err != nil {
			successfulUsers--
		}
	}

	// ignore errors
	b.addCompletedTokens(job, successfulUsers)
	b.addCompletedBatch(job)
	complete, _ := b.checkComplete(job)
	if complete {
		job.CompletedAt = time.Now().UnixNano()
		_, err = b.Workers.MarathonDB.DB.Model(&job).Column("completed_at").Update()
	}
}

func (b *DirectWorker) checkErr(job *model.Job, err error) {
	if err != nil {
		job.TagError(b.Workers.MarathonDB, nameDirectWorker, err.Error())
		checkErr(b.Logger, err)
	}
}
