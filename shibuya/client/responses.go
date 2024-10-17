package client

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

func handleResponse(resp *http.Response, obj any) error {
	switch status := resp.StatusCode; {
	case status >= 400:
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Println(string(raw))
		return errors.New(resp.Status)
	case status == 200:
		if obj != nil {
			if err := json.NewDecoder(resp.Body).Decode(obj); err != nil {
				return err
			}
		}

	}
	return nil
}
