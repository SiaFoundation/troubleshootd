package api

import "time"

// StateResponse is the response for the GET /state endpoint.
type StateResponse struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	OS        string    `json:"os"`
	BuildTime time.Time `json:"buildTime"`
}
