package util

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func ClientRequest(client *http.Client, method, url string, headers map[string]string, data []byte) (int, []byte, error) {
	var res []byte
	var code = 0
	req, err := http.NewRequest(strings.ToUpper(method), url, bytes.NewBuffer(data))
	if err != nil {
		return code, res, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	//req.AddCookie(&http.Cookie{Name: "session", Value: ""})
	response, err := client.Do(req)
	if err != nil && response == nil {
		return code, res, fmt.Errorf("error: %+v", err)
	} else {
		if response != nil {
			defer response.Body.Close()
			r, err := io.ReadAll(response.Body)
			return response.StatusCode, r, err
		}
		return code, res, nil
	}
}

func PostForm(reqUrl string, content map[string]string) (int, []byte, error) {
	data := url.Values{}
	for k, v := range content {
		data.Add(k, v)
	}

	resp, err := http.PostForm(reqUrl, data)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return 0, nil, err
	}
	r, err := io.ReadAll(resp.Body)
	return resp.StatusCode, r, err
}
