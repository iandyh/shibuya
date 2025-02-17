package apitests

import "os"

func getEndpoint() string {
	e := os.Getenv("endpoint")
	if e == "" {
		return "http://localhost:8080"
	}
	return e
}
