/*
   Copyright (c) 2016, Percona LLC and/or its affiliates. All rights reserved.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package test

import (
	"io/ioutil"
	"net/http"
)

type Client struct {
	client  *http.Client
	Headers map[string]string
}

func NewClient() *Client {
	client := &http.Client{}
	c := &Client{
		client:  client,
		Headers: make(map[string]string),
	}
	return c
}

func (c *Client) Get(url string) (*http.Response, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)

	for h, v := range c.Headers {
		req.Header.Add(h, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return resp, nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, nil, err
	}
	return resp, content, nil
}
