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

package model

import (
	"github.com/asaskevich/govalidator"
	"github.com/satori/go.uuid"
)

// JobGroup is a collection of jobs
type JobGroup struct {
	ID               uuid.UUID              `sql:",pk" json:"id"`
	AppID            uuid.UUID              `json:"appId"`
	CreatedAt        int64                  `json:"createdAt"`
	Context          map[string]interface{} `json:"context"`
	Metadata         map[string]interface{} `json:"metadata"`
	TemplateName     string                 `json:"templateName"`
	ControlGroup     float64                `json:"controlGroup"`
	CreatedBy        string                 `json:"createdBy"`
	CSVPath          string                 `json:"csvPath"`
	App              *App                   `json:"app"`
	Jobs             []*Job                 `json:"jobs"`
	Localized        bool                   `json:"localized"`
	PastTimeStrategy string                 `json:"pastTimeStrategy"`
}

// Validate implementation of the InputValidation interface
func (j *JobGroup) Validate() error {
	valid := j.ControlGroup >= 0 && j.ControlGroup < 1
	if !valid {
		return InvalidField("controlGroup")
	}

	valid = govalidator.IsEmail(j.CreatedBy)
	if !valid {
		return InvalidField("createdBy")
	}

	if !govalidator.IsNull(j.CSVPath) && govalidator.Contains(j.CSVPath, "s3://") {
		return InvalidField("csvPath: cannot contain s3 protocol, just the bucket path")
	}
	return nil
}
