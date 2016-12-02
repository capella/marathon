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
	"io/ioutil"

	"github.com/labstack/echo"
)

// RecordNotFoundString is the string returned when a record is not found
var RecordNotFoundString = "record not found"

//Error is a struct to help return errors
type Error struct {
	Reason string          `json:"reason"`
	Value  InputValidation `json:"value"`
}

//GetRequestBody from echo context
func GetRequestBody(c echo.Context) ([]byte, error) {
	bodyCache := c.Get("requestBody")
	if bodyCache != nil {
		return bodyCache.([]byte), nil
	}
	b, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	c.Set("requestBody", b)
	return b, nil
}