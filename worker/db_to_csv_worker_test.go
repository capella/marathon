/*
 * Copyright (c) 2016 TFG Co <backend@tfgco.com>
 * Author: TFG Co <backend@tfgco.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package worker_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	workers "github.com/jrallison/go-workers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/model"
	. "github.com/topfreegames/marathon/testing"
	"github.com/topfreegames/marathon/worker"
	"github.com/uber-go/zap"
)

var _ = Describe("CreateBatchesFromFilters Worker", func() {
	var app *model.App
	var template *model.Template

	logger := zap.New(
		zap.NewJSONEncoder(zap.NoTime()),
		zap.FatalLevel,
	)
	w := worker.NewWorker(logger, GetConfPath())
	createBatchesFromFiltersWorker := worker.NewCreateBatchesFromFiltersWorker(w)
	createDbToCsvBatchesWorker := worker.NewDbToCsvWorker(w)

	BeforeEach(func() {
		app = CreateTestApp(w.MarathonDB.DB)
		defaults := map[string]interface{}{
			"user_name":   "Someone",
			"object_name": "village",
		}
		body := map[string]interface{}{
			"alert": "{{user_name}} just liked your {{object_name}}!",
		}
		template = CreateTestTemplate(w.MarathonDB.DB, app.ID, map[string]interface{}{
			"defaults": defaults,
			"body":     body,
			"locale":   "en",
		})
		users := make([]worker.User, 2)
		for index := range users {
			id := uuid.NewV4().String()
			token := strings.Replace(uuid.NewV4().String(), "-", "", -1)
			users[index] = worker.User{
				UserID: id,
				Token:  token,
				Locale: "en",
			}
		}
		w.RedisClient.FlushAll()
	})

	Describe("Process", func() {
		It("should generate a csv with the right number of users", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "en",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(5))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("57be9009-e616-42c6-9cfe-505508ede2d0"))
			Expect(lines).To(ContainElement("5c3033c0-24ad-487a-a80d-68432464c8de"))
			Expect(lines).To(ContainElement("2df5bb01-15d1-4569-bc56-49fa0a33c4c3"))
			Expect(lines).To(ContainElement("21854bbf-ea7e-43e3-8f79-9ab2c121b941"))

		})

		It("should generate a csv with the right number of users if the job contains a filter with multiple values separated by comma", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "en,pt",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(11))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("57be9009-e616-42c6-9cfe-505508ede2d0"))
			Expect(lines).To(ContainElement("5c3033c0-24ad-487a-a80d-68432464c8de"))
			Expect(lines).To(ContainElement("2df5bb01-15d1-4569-bc56-49fa0a33c4c3"))
			Expect(lines).To(ContainElement("21854bbf-ea7e-43e3-8f79-9ab2c121b941"))
			Expect(lines).To(ContainElement("9e558649-9c23-469d-a11c-59b05813e3d5"))
			Expect(lines).To(ContainElement("a8e8d2d5-f178-4d90-9b31-683ad3aae920"))
			Expect(lines).To(ContainElement("4223171e-c665-4612-9edd-485f229240bf"))
			Expect(lines).To(ContainElement("67b872de-8ae4-4763-aef8-7c87a7f928a7"))
			Expect(lines).To(ContainElement("3f8732a1-8642-4f22-8d77-a9688dd6a5ae"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-9ab7-8becb2765653"))
			Expect(lines).NotTo(ContainElement("843a61f8-45b3-44f9-9ab7-8becb3365653"))
		})

		It("should generate a csv with the right number of users if the job contains a filter with multiple values separated by comma", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"tz": "-0500,-0800",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(10))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("5c3033c0-24ad-487a-a80d-68432464c8de"))
			Expect(lines).To(ContainElement("67b872de-8ae4-4763-aef8-7c87a7f928a7"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-9ab7-8becb2765653"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-9ab7-8becb3365653"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-aaaa-8becb3365653"))
			Expect(lines).To(ContainElement("e78431ca-69a8-4326-af1f-48f817a4a669"))
			Expect(lines).To(ContainElement("d9b42bb8-78ca-44d0-ae50-a472d9fbad92"))
			Expect(lines).To(ContainElement("ee4455fe-8ff6-4878-8d7c-aec096bd68b4"))
		})

		It("should generate a csv with the right users", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "pt",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(7))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("9e558649-9c23-469d-a11c-59b05813e3d5"))
			Expect(lines).To(ContainElement("a8e8d2d5-f178-4d90-9b31-683ad3aae920"))
			Expect(lines).To(ContainElement("4223171e-c665-4612-9edd-485f229240bf"))
			Expect(lines).To(ContainElement("67b872de-8ae4-4763-aef8-7c87a7f928a7"))
			Expect(lines).To(ContainElement("3f8732a1-8642-4f22-8d77-a9688dd6a5ae"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-9ab7-8becb2765653"))
		})

		It("should generate a csv with the right number of users", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "au",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(2))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("843a61f8-45b3-44f9-9ab7-8becb3365653"))
		})

		It("should generate a csv with the right number of users if using 2 filters", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "pt",
					"tz":     "-0300",
				},
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(5))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("9e558649-9c23-469d-a11c-59b05813e3d5"))
			Expect(lines).To(ContainElement("a8e8d2d5-f178-4d90-9b31-683ad3aae920"))
			Expect(lines).To(ContainElement("4223171e-c665-4612-9edd-485f229240bf"))
			Expect(lines).To(ContainElement("3f8732a1-8642-4f22-8d77-a9688dd6a5ae"))
		})

		It("should generate a csv with the right number of users if using 2 filters", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "PT",
					"tz":     "-0300",
				},
				"service": "gcm",
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
			generatedCSV, err := fakeS3.GetObject(key)
			Expect(err).NotTo(HaveOccurred())
			lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
			Expect(len(lines)).To(Equal(5))
			Expect(lines).To(ContainElement("userIds"))
			Expect(lines).To(ContainElement("9e558649-9c23-469d-a11c-59b05000e3d5"))
			Expect(lines).To(ContainElement("a8e8d2d5-f178-4d90-9b31-683ad3aae920"))
			Expect(lines).To(ContainElement("4223171e-c665-4612-9edd-485f229240bf"))
			Expect(lines).To(ContainElement("3f8732a1-8642-4f22-8d77-a9688dd6a5ae"))
		})

		It("should enqueue a createBatchesWorker with the right jobID", func() {
			a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
			j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
				"filters": map[string]interface{}{
					"locale": "PT",
					"tz":     "-0300",
				},
				"service": "gcm",
			})
			fakeS3 := NewFakeS3(w.Config)
			w.S3Client = fakeS3
			m := map[string]interface{}{
				"jid":  6,
				"args": []string{j.ID.String()},
			}
			smsg, err := json.Marshal(m)
			Expect(err).NotTo(HaveOccurred())
			msg, err := workers.NewMsg(string(smsg))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

			jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
			msg, err = workers.NewMsg(string(jobData))
			Expect(err).NotTo(HaveOccurred())
			Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

			res, err := w.RedisClient.LLen("queue:csv_split_worker").Result()
			Expect(res).To(BeEquivalentTo(1))
			j1 := map[string]interface{}{}
			job1, err := w.RedisClient.LPop("queue:csv_split_worker").Result()
			err = json.Unmarshal([]byte(job1), &j1)
			Expect(err).NotTo(HaveOccurred())
			Expect(j1["queue"].(string)).To(Equal("csv_split_worker"))
			Expect(j1["args"].(string)).To(Equal(j.ID.String()))
		})
	})

	It("should generate a csv without duplicates", func() {
		a := CreateTestApp(w.MarathonDB.DB, map[string]interface{}{"name": "testapp"})
		j := CreateTestJob(w.MarathonDB.DB, a.ID, template.Name, map[string]interface{}{
			"filters": map[string]interface{}{
				"locale": "es",
			},
			"service": "apns",
		})
		fakeS3 := NewFakeS3(w.Config)
		w.S3Client = fakeS3
		m := map[string]interface{}{
			"jid":  7,
			"args": []string{j.ID.String()},
		}
		smsg, err := json.Marshal(m)
		Expect(err).NotTo(HaveOccurred())
		msg, err := workers.NewMsg(string(smsg))
		Expect(err).NotTo(HaveOccurred())
		Expect(func() { createBatchesFromFiltersWorker.Process(msg) }).ShouldNot(Panic())

		jobData, err := w.RedisClient.LPop("queue:db_to_csv_worker").Result()
		msg, err = workers.NewMsg(string(jobData))
		Expect(err).NotTo(HaveOccurred())
		Expect(func() { createDbToCsvBatchesWorker.Process(msg) }).ShouldNot(Panic())

		key := fmt.Sprintf("%s/%s/job-%s.csv", w.Config.GetString("s3.bucket"), w.Config.GetString("s3.folder"), j.ID)
		generatedCSV, err := fakeS3.GetObject(key)
		Expect(err).NotTo(HaveOccurred())
		lines := ReadLinesFromIOReader(bytes.NewReader(generatedCSV))
		Expect(len(lines)).To(Equal(4))
	})
})
