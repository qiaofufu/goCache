package utls

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

/* HTTP 请求封装 */

func Get(url string) (*http.Response, error) {
	return Request(http.MethodGet, url, nil)
}

func Delete(url string) (*http.Response, error) {
	return Request(http.MethodDelete, url, nil)
}

func Post(url string, body []byte) (*http.Response, error) {
	reader := bytes.NewReader(body)
	return Request(http.MethodPost, url, reader)
}

func Request(method string, url string, body io.Reader) (resp *http.Response, err error) {
	client := http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to new request, err: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to sent request, err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("faield to send request, status: %s", resp.Status)
	}
	return
}
