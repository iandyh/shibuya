package client

import (
	"errors"
	"io"
	"log"
	"net/http"
)

func handleResponse(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		// TODO We can marshal the api error message once the api package can be imported
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Println(string(raw))
		return errors.New(resp.Status)
	}
	return nil
}
