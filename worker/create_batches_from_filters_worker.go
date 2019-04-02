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
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
	workers "github.com/jrallison/go-workers"
	uuid "github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/model"
	"github.com/uber-go/zap"
)

const nameCreateBatchesFromFilters = "create_batches_using_filters_worker"

// CreateBatchesFromFiltersWorker is the CreateBatchesUsingFiltersWorker struct
type CreateBatchesFromFiltersWorker struct {
	Workers *Worker
	Logger  zap.Logger
}

// NewCreateBatchesFromFiltersWorker gets a new CreateBatchesFromFiltersWorker
func NewCreateBatchesFromFiltersWorker(workers *Worker) *CreateBatchesFromFiltersWorker {
	b := &CreateBatchesFromFiltersWorker{
		Logger:  workers.Logger.With(zap.String("worker", "CreateBatchesFromFiltersWorker")),
		Workers: workers,
	}
	b.Logger.Debug("Configured CreateBatchesFromFiltersWorker successfully")
	return b
}

func (b *CreateBatchesFromFiltersWorker) createDbToCsvJob(job *model.Job, page DBPage,
	uploader *s3.CreateMultipartUploadOutput, total int) {
	filters := job.Filters
	whereClause := GetWhereClauseFromFilters(filters)
	limit := page.DBPageSize
	var query string
	if (whereClause) != "" {
		query = fmt.Sprintf("SELECT DISTINCT user_id FROM %s WHERE user_id > '%s' AND %s ORDER BY user_id LIMIT %d;", GetPushDBTableName(job.App.Name, job.Service), page.SmallestID, whereClause, limit)
	} else {
		query = fmt.Sprintf("SELECT DISTINCT user_id FROM %s WHERE user_id > '%s' ORDER BY user_id LIMIT %d;", GetPushDBTableName(job.App.Name, job.Service), page.SmallestID, limit)
	}
	b.Workers.DbToCsvJob(&ToCSVMenssage{
		Query:      query,
		PartNumber: page.Page,
		Uploader:   *uploader,
		TotalJobs:  total,
		Job:        *job,
	})
}

func (b *CreateBatchesFromFiltersWorker) preprocessPages(job *model.Job) ([]DBPage, int, int) {
	filters := job.Filters
	var count int
	whereClause := GetWhereClauseFromFilters(filters)
	var query string
	if (whereClause) != "" {
		query = fmt.Sprintf("SELECT count(DISTINCT user_id) FROM %s WHERE %s", GetPushDBTableName(job.App.Name, job.Service), whereClause)
	} else {
		query = fmt.Sprintf("SELECT count(DISTINCT user_id) FROM %s", GetPushDBTableName(job.App.Name, job.Service))
	}

	labels := []string{fmt.Sprintf("game:%s", job.App.Name), fmt.Sprintf("platform:%s", job.Service)}
	start := time.Now()

	_, err := b.Workers.PushDB.DB.Query(&count, query)
	if count == 0 {
		checkErr(b.Logger, fmt.Errorf("no users matching the filters"))
	}
	b.Workers.Statsd.Timing("count_total", time.Now().Sub(start), labels, 1)

	// The maximum of ids to fill one multiupload part (in mb)
	megaBytes := b.Workers.Config.GetInt("workers.amazonPartSize")
	DBPageSize := int(math.Ceil((float64(megaBytes) * 1024.0 * 1024.0) / 37.0))

	pageCount := int(math.Ceil(float64(count) / float64(DBPageSize)))
	checkErr(b.Logger, err)
	pages := []DBPage{}
	nextPageOffset := "00000000-0000-0000-0000-000000000000"

	start = time.Now()

	tx, err := b.Workers.PushDB.DB.Begin()
	checkErr(b.Logger, err)

	defer tx.Rollback()

	if (whereClause) != "" {
		query = fmt.Sprintf("DECLARE cursor CURSOR FOR SELECT DISTINCT user_id FROM %s WHERE %s ORDER BY user_id;", GetPushDBTableName(job.App.Name, job.Service), whereClause)
	} else {
		query = fmt.Sprintf("DECLARE cursor CURSOR FOR SELECT DISTINCT user_id FROM %s ORDER BY user_id;", GetPushDBTableName(job.App.Name, job.Service))
	}
	tx.Query(nil, query)

	for page := 0; page < pageCount; page++ {
		pages = append(pages, DBPage{
			Page:       page + 1, // amazon page start at 1
			SmallestID: nextPageOffset,
			DBPageSize: DBPageSize,
		})
		_, err := tx.Query(&nextPageOffset, fmt.Sprintf("FETCH RELATIVE +%d FROM cursor;", DBPageSize))
		checkErr(b.Logger, err)
	}
	tx.Commit()
	b.Workers.Statsd.Timing("get_intervals_cursor", time.Now().Sub(start), labels, 1)

	return pages, pageCount, count
}

func (b *CreateBatchesFromFiltersWorker) createBatchesFromFilters(job *model.Job) {
	pages, pageCount, _ := b.preprocessPages(job)

	folder := b.Workers.Config.GetString("s3.folder")
	writePath := fmt.Sprintf("%s/job-%s.csv", folder, job.ID.String())
	uploader, err := b.Workers.S3Client.InitMultipartUpload(writePath)
	checkErr(b.Logger, err)
	b.updateJobCSVPath(job, writePath)

	for i := 0; i < pageCount; i++ {
		b.createDbToCsvJob(job, pages[i], uploader, pageCount)
	}
}

func (b *CreateBatchesFromFiltersWorker) updateJobCSVPath(job *model.Job, csvPath string) {
	job.CSVPath = csvPath
	_, err := b.Workers.MarathonDB.DB.Model(job).Set("csv_path = ?csv_path").Update()
	checkErr(b.Logger, err)
}

// Process processes the messages sent to worker queue
func (b *CreateBatchesFromFiltersWorker) Process(message *workers.Msg) {
	arr, err := message.Args().Array()
	checkErr(b.Logger, err)
	jobID := arr[0]
	id, err := uuid.FromString(jobID.(string))
	checkErr(b.Logger, err)
	l := b.Logger.With(
		zap.String("jobID", id.String()),
		zap.String("worker", nameCreateBatchesFromFilters),
	)
	l.Info("starting")

	job := &model.Job{
		ID: id,
	}
	job.TagRunning(b.Workers.MarathonDB, nameCreateBatchesFromFilters, "starting")
	err = b.Workers.MarathonDB.DB.Model(job).Column("job.*", "App").Where("job.id = ?", job.ID).Select()
	if err != nil {
		job.TagError(b.Workers.MarathonDB, nameCreateBatchesFromFilters, err.Error())
		return
	}
	if job.Status == stoppedJobStatus {
		l.Info("stopped job")
		return
	}

	b.createBatchesFromFilters(job)

	job.TagSuccess(b.Workers.MarathonDB, nameCreateBatchesFromFilters, "finished")
	l.Info("finished")
}

func (b *CreateBatchesFromFiltersWorker) checkErr(job *model.Job, err error) {
	if err != nil {
		job.TagError(b.Workers.MarathonDB, nameCreateBatchesFromFilters, err.Error())
		checkErr(b.Logger, err)
	}
}
