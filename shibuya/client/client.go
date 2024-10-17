package client

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func makeFormValues(params map[string]string) url.Values {
	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}
	return values
}

func makeFormRequest(resourceUrl string, params map[string]string) (*http.Request, error) {
	values := makeFormValues(params)
	req, err := http.NewRequest("POST", resourceUrl, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func makeFileUploadRequest(resourceUrl, method string, formName string, file *os.File) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(formName, file.Name())
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, resourceUrl, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}
