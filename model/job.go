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

package model

import (
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/interfaces"
)

// Job is the job model struct
type Job struct {
	ID                  uuid.UUID              `sql:",pk" json:"id"`
	TotalBatches        int                    `json:"totalBatches"`
	CompletedBatches    int                    `json:"completedBatches"`
	TotalTokens         int                    `json:"totalTokens"`
	TotalUsers          int                    `json:"totalUsers"`
	CompletedTokens     int                    `json:"completedTokens"`
	DBPageSize          int                    `json:"dbPageSize"`
	CompletedAt         int64                  `json:"completedAt"`
	ExpiresAt           int64                  `json:"expiresAt"`
	StartsAt            int64                  `json:"startsAt"`
	Service             string                 `json:"service"`
	Filters             map[string]interface{} `json:"filters"`
	ControlGroupCSVPath string                 `json:"controlGroupCsvPath"`
	JobGroupID          uuid.UUID              `json:"jobGroupId" sql:",null"`
	Status              string                 `json:"status"`
	Feedbacks           map[string]interface{} `json:"feedbacks"`
	UpdatedAt           int64                  `json:"updatedAt"`
	StatusEvents        []*Status              `json:"statusEvents"`
	JobGroup            *JobGroup              `json:"jobGroup"`
}

// Validate implementation of the InputValidation interface
func (j *Job) Validate() error {
	valid := govalidator.StringMatches(j.Service, "^(apns|gcm)$")
	if !valid {
		return InvalidField("service")
	}

	valid = j.ExpiresAt == 0 || time.Now().UnixNano() < j.ExpiresAt
	if !valid {
		return InvalidField("expiresAt")
	}

	valid = !(len(j.Filters) != 0 && !govalidator.IsNull(j.JobGroup.CSVPath))
	if !valid {
		return InvalidField("filters or csvPath must exist, not both")
	}

	return nil
}

// Labels return the labels for metrics
func (j *Job) Labels() []string {
	return []string{
		fmt.Sprintf("game:%s", j.JobGroup.App.Name),
		fmt.Sprintf("platform:%s", j.Service),
	}
}

func (j *Job) tag(db interfaces.DB, name, message, state string) {
	status := &Status{
		Name:      name,
		JobID:     j.ID,
		ID:        uuid.NewV4(),
		CreatedAt: time.Now().UnixNano(),
	}
	_, err := db.Model(status).OnConflict("(name, job_id) DO UPDATE").Set("name = EXCLUDED.name").Returning("id").Insert()
	if err != nil {
		panic(err)
	}
	event := &Events{
		Message:   message,
		StatusID:  status.ID,
		State:     state,
		ID:        uuid.NewV4(),
		CreatedAt: time.Now().UnixNano(),
	}
	_, err = db.Model(event).Insert()
	if err != nil {
		panic(err)
	}
}

// TagSuccess create a status in one job
func (j *Job) TagSuccess(db interfaces.DB, name, message string) {
	j.tag(db, name, message, "success")
}

// TagError create a status in one job
func (j *Job) TagError(db interfaces.DB, name, message string) {
	j.tag(db, name, message, "fail")
}

// TagRunning create a status in one job
func (j *Job) TagRunning(db interfaces.DB, name, message string) {
	j.tag(db, name, message, "running")
}
