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

package testing

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/pg.v5"

	"github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/model"
)

func getOpt(options map[string]interface{}, key string, defaultValue interface{}) interface{} {
	val, ok := options[key]
	if !ok {
		val = defaultValue
	}

	return val
}

//CreateTestApp with specified optional values
func CreateTestApp(db *pg.DB, options ...map[string]interface{}) *model.App {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}

	app := &model.App{}
	app.ID = getOpt(opts, "id", uuid.NewV4()).(uuid.UUID)
	app.Name = getOpt(opts, "name", uuid.NewV4().String()).(string)
	app.BundleID = getOpt(opts, "bundleId", fmt.Sprintf("com.app.%s", strings.Split(uuid.NewV4().String(), "-")[0])).(string)
	app.CreatedBy = getOpt(opts, "createdBy", "test@test.com").(string)

	err := db.Insert(&app)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return app
}

//CreateTestApps for n apps
func CreateTestApps(db *pg.DB, n int, options ...map[string]interface{}) []*model.App {
	apps := make([]*model.App, n)
	for i := 0; i < n; i++ {
		app := CreateTestApp(db, options...)
		apps[i] = app
	}

	return apps
}

//GetAppPayload with specified optional values
func GetAppPayload(options ...map[string]interface{}) map[string]interface{} {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}
	id := getOpt(opts, "id", uuid.NewV4())
	name := getOpt(opts, "name", uuid.NewV4().String()).(string)
	bundleID := getOpt(opts, "bundleId", fmt.Sprintf("com.app.%s", strings.Split(uuid.NewV4().String(), "-")[0])).(string)

	app := map[string]interface{}{
		"id":       id,
		"name":     name,
		"bundleId": bundleID,
	}
	return app
}

//CreateTestTemplate with specified optional values
func CreateTestTemplate(db *pg.DB, appID uuid.UUID, options ...map[string]interface{}) *model.Template {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}

	defaults := getOpt(opts, "defaults", map[string]string{"value": "default"}).(map[string]string)
	body := getOpt(opts, "body", map[string]string{"value": "custom"}).(map[string]string)

	template := &model.Template{}
	template.AppID = appID
	template.Defaults = defaults
	template.Body = body
	template.ID = getOpt(opts, "id", uuid.NewV4()).(uuid.UUID)
	template.Name = getOpt(opts, "name", uuid.NewV4().String()).(string)
	template.Locale = getOpt(opts, "locale", strings.Split(uuid.NewV4().String(), "-")[0]).(string)
	template.CompiledBody = getOpt(opts, "compiledBody", "compiled-body").(string)
	template.CreatedBy = getOpt(opts, "createdBy", "test@test.com").(string)

	err := db.Insert(&template)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return template
}

//CreateTestTemplates for n apps
func CreateTestTemplates(db *pg.DB, appID uuid.UUID, n int, options ...map[string]interface{}) []*model.Template {
	templates := make([]*model.Template, n)
	for i := 0; i < n; i++ {
		template := CreateTestTemplate(db, appID, options...)
		templates[i] = template
	}

	return templates
}

//GetTemplatePayload with specified optional values
func GetTemplatePayload(options ...map[string]interface{}) map[string]interface{} {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}

	name := getOpt(opts, "name", uuid.NewV4().String()).(string)
	id := getOpt(opts, "id", uuid.NewV4()).(uuid.UUID)
	locale := getOpt(opts, "locale", strings.Split(uuid.NewV4().String(), "-")[0]).(string)

	pl, _ := json.Marshal(getOpt(opts, "defaults", map[string]string{"value": "default"}).(map[string]string))
	defaults := string(pl)
	pl, _ = json.Marshal(getOpt(opts, "body", map[string]string{"value": "custom"}).(map[string]string))
	body := string(pl)

	template := map[string]interface{}{
		"name":     name,
		"locale":   locale,
		"defaults": defaults,
		"body":     body,
		"id":       id,
	}
	return template
}

//CreateTestJob with specified optional values
func CreateTestJob(db *pg.DB, appID uuid.UUID, templateID uuid.UUID, options ...map[string]interface{}) *model.Job {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}

	filters := getOpt(opts, "filters", map[string]string{"locale": "en"}).(map[string]string)
	context := getOpt(opts, "context", map[string]string{"value": "context"}).(map[string]string)

	job := &model.Job{}
	job.AppID = appID
	job.TemplateID = templateID
	job.Filters = filters
	job.Context = context
	job.ID = getOpt(opts, "id", uuid.NewV4()).(uuid.UUID)
	job.Service = getOpt(opts, "service", "apns").(string)
	job.CsvURL = getOpt(opts, "csvUrl", "").(string)
	job.ExpiresAt = getOpt(opts, "expiresAt", time.Now().Add(time.Hour)).(time.Time)
	job.CreatedBy = getOpt(opts, "createdBy", "test@test.com").(string)

	err := db.Insert(&job)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return job
}

//CreateTestJobs for n apps
func CreateTestJobs(db *pg.DB, appID uuid.UUID, templateID uuid.UUID, n int, options ...map[string]interface{}) []*model.Job {
	jobs := make([]*model.Job, n)
	for i := 0; i < n; i++ {
		job := CreateTestJob(db, appID, templateID, options...)
		jobs[i] = job
	}

	return jobs
}

//GetJobPayload with specified optional values
func GetJobPayload(options ...map[string]interface{}) map[string]interface{} {
	opts := map[string]interface{}{}
	if len(options) == 1 {
		opts = options[0]
	}

	pl, _ := json.Marshal(getOpt(opts, "filters", map[string]string{"locale": "en"}).(map[string]string))
	filters := string(pl)
	pl, _ = json.Marshal(getOpt(opts, "context", map[string]string{"value": "context"}).(map[string]string))
	context := string(pl)

	service := getOpt(opts, "service", "apns").(string)
	csvURL := getOpt(opts, "csvUrl", "").(string)
	expiresAt := getOpt(opts, "expiresAt", time.Now().Add(time.Hour)).(time.Time)
	id := getOpt(opts, "id", uuid.NewV4()).(uuid.UUID)

	job := map[string]interface{}{
		"filters":   filters,
		"context":   context,
		"service":   service,
		"csvUrl":    csvURL,
		"expiresAt": expiresAt,
		"id":        id,
	}
	return job
}