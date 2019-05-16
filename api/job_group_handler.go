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

package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo"
	"github.com/satori/go.uuid"
	"github.com/topfreegames/marathon/log"
	"github.com/topfreegames/marathon/model"
	"github.com/uber-go/zap"
)

// ListGroupsHandler is the method called when a get to /groups is called
func (a *Application) ListGroupsHandler(c echo.Context) error {
	l := a.Logger.With(
		zap.String("source", "ListGroupsHandler"),
		zap.String("operation", "listGroups"),
	)

	appID, err := uuid.FromString(c.Param("aid"))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, &Error{Reason: err.Error()})
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		page = 0
	}
	// TODO: hub api is not passing this parameter to the api
	limit, err := strconv.Atoi(c.QueryParam("limit"))
	if err != nil {
		limit = 15
	}

	// get groups in page
	groups := []model.JobGroup{}
	err = WithSegment("db-select", c, func() error {
		return a.DB.Model(&groups).
			Column("job_group.*", "Jobs", "Jobs.StatusEvents", "Jobs.StatusEvents.Events").
			Where("job_group.app_id = ?", appID).
			Order("created_at DESC").
			Limit(limit).
			Offset(page * limit).
			Select()
	})
	if err != nil {
		log.E(l, "Failed to list groups.", func(cm log.CM) {
			cm.Write(zap.Error(err))
		})
		return c.JSON(http.StatusInternalServerError, &Error{Reason: err.Error()})
	}

	// count total groups
	total, err := a.DB.Model(&groups).
		Where("job_group.app_id = ?", appID).
		Count()
	if err != nil {
		log.E(l, "Failed to list groups.", func(cm log.CM) {
			cm.Write(zap.Error(err))
		})
		return c.JSON(http.StatusInternalServerError, &Error{Reason: err.Error()})
	}

	return c.JSON(http.StatusOK, struct {
		Groups []model.JobGroup `json:"groups"`
		Total  int              `json:"total"`
	}{
		groups,
		total,
	})
}

// GetGroupHandler is the method called when a get to /group/:gid is called
func (a *Application) GetGroupHandler(c echo.Context) error {
	l := a.Logger.With(
		zap.String("source", "GetGroupsHandler"),
		zap.String("operation", "listGroups"),
	)

	appID, err := uuid.FromString(c.Param("aid"))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, &Error{Reason: err.Error()})
	}

	groupID, err := uuid.FromString(c.Param("gid"))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, &Error{Reason: err.Error()})
	}

	// get groups in page
	group := &model.JobGroup{}
	err = WithSegment("db-select", c, func() error {
		return a.DB.Model(group).
			Column("job_group.*", "Jobs", "Jobs.StatusEvents", "Jobs.StatusEvents.Events").
			Where("job_group.id = ? AND job_group.app_id = ?", groupID, appID).
			Order("created_at DESC").
			Select()
	})
	if err != nil {
		log.E(l, "Failed to get group groups.", func(cm log.CM) {
			cm.Write(zap.Error(err))
		})
		return c.JSON(http.StatusInternalServerError, &Error{Reason: err.Error()})
	}

	return c.JSON(http.StatusOK, group)
}
