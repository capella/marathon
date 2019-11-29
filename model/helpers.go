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
	"strings"

	"github.com/topfreegames/marathon/interfaces"
	pg "gopkg.in/pg.v5"
)

// InvalidField returns an error telling that field is invalid
func InvalidField(field string) error {
	return fmt.Errorf("invalid %s", field)
}

// GetJobInfoAndApp get the app and the job from the database
// job.ID must be set
func (j *Job) GetJobInfoAndApp(db interfaces.DB) error {
	err := db.Select(j)
	if err != nil {
		return err
	}
	j.JobGroup = &JobGroup{}
	err = db.Model(j.JobGroup).Where("id = ?", j.JobGroupID).Select()
	if err != nil {
		return err
	}
	j.JobGroup.App = &App{}
	return db.Model(j.JobGroup.App).Where("id = ?", j.JobGroup.AppID).Select()
}

// GetJobTemplatesByNameAndLocale ...
func (j *Job) GetJobTemplatesByNameAndLocale(db interfaces.DB) (map[string]map[string]Template, error) {
	var templates []Template
	var err error
	if len(strings.Split(j.JobGroup.TemplateName, ",")) > 1 {
		err = db.Model(&templates).Where(
			"app_id = ? AND name IN (?)",
			j.JobGroup.App.ID,
			pg.In(strings.Split(j.JobGroup.TemplateName, ",")),
		).Select()
	} else {
		err = db.Model(&templates).Where(
			"app_id = ? AND name = ?",
			j.JobGroup.App.ID,
			j.JobGroup.TemplateName,
		).Select()
	}
	if err != nil {
		return nil, err
	}
	templateByLocale := make(map[string]map[string]Template)
	for _, tpl := range templates {
		if templateByLocale[tpl.Name] != nil {
			templateByLocale[tpl.Name][tpl.Locale] = tpl
		} else {
			templateByLocale[tpl.Name] = map[string]Template{
				tpl.Locale: tpl,
			}
		}
	}

	if len(templateByLocale) == 0 {
		return nil, fmt.Errorf("No templates were found with name %s", j.JobGroup.TemplateName)
	}
	return templateByLocale, nil
}

// GetQuery ...
func (j *Job) GetQuery() string {
	filters := j.Filters
	whereClause := GetWhereClauseFromFilters(filters)
	query := fmt.Sprintf("SELECT user_id, token, locale, tz FROM %s_%s WHERE seq_id >= ? AND seq_id < ?", j.JobGroup.App.Name, j.Service)
	if (whereClause) != "" {
		query = fmt.Sprintf("%s AND %s", query, whereClause)
	}
	return query
}

// PredictQuery ...
func (j *Job) PredictQuery() string {
	filters := j.Filters
	whereClause := GetWhereClauseFromFilters(filters)
	query := fmt.Sprintf("SELECT user_id, token, locale, tz FROM %s_%s", j.JobGroup.App.Name, j.Service)
	if (whereClause) != "" {
		query = fmt.Sprintf("%s WHERE %s", query, whereClause)
	}
	return query
}

// GetWhereClauseFromFilters returns a string cointaining the where clause to use in the query
func GetWhereClauseFromFilters(filters map[string]interface{}) string {
	if len(filters) == 0 {
		return ""
	}

	queryFilters := []string{}
	for key, val := range filters {
		operator := "="
		connector := " OR "
		if strings.Contains(key, "NOT") {
			key = strings.Trim(key, "NOT")
			operator = "!="
			connector = " AND "
		}
		strVal := val.(string)
		if strings.Contains(strVal, ",") {
			filterArray := []string{}
			vals := strings.Split(strVal, ",")
			for _, fVal := range vals {
				filterArray = append(filterArray, fmt.Sprintf("\"%s\"%s'%s'", key, operator, fVal))
			}
			queryFilters = append(queryFilters, fmt.Sprintf("(%s)", strings.Join(filterArray, connector)))
		} else {
			queryFilters = append(queryFilters, fmt.Sprintf("\"%s\"%s'%s'", key, operator, val))
		}
	}
	return strings.Join(queryFilters, " AND ")
}
