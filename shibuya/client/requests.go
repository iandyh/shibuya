package client

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/model"
)

type ShibuyaObject interface {
	*model.Project | *model.Collection | *model.Plan
}

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

func sendCreateRequest[T ShibuyaObject](client *http.Client, resourceUrl string, params map[string]string, obj T) (T, error) {
	req, err := makeFormRequest(resourceUrl, params)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := handleResponse(resp, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func sendDeleteRequest(client *http.Client, resourceUrl string) error {
	req, err := http.NewRequest("DELETE", resourceUrl, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func sendGetRequest[T ShibuyaObject](client *http.Client, resourceUrl string, obj T) (T, error) {
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := handleResponse(resp, obj); err != nil {
		return nil, err
	}
	return obj, nil
}
