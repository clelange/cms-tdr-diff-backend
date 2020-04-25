package main

import (
	"fmt"
	"net/http"
	"time"
)

func (s *server) handlePing() http.HandlerFunc {
	type response struct {
		Message string `json:"message"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		pongResponse := response{
			Message: "pong",
		}
		respond(w, r, http.StatusOK, pongResponse)
	}
}

func (s *server) handleLastUpdated() http.HandlerFunc {
	type response struct {
		LastUpdated time.Time `json:"lastUpdated"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		lastUpdatedResponse := response{
			LastUpdated: lastUpdated,
		}
		respond(w, r, http.StatusOK, lastUpdatedResponse)
	}
}

func (s *server) handleVersion() http.HandlerFunc {
	type response struct {
		URL         string   `json:"URL"`
		Method      string   `json:"Method"`
		Headers     []string `json:"Headers"`
		Version     string   `json:"Version"`
		BuildTime   string   `json:"BuildTime"`
		SnapshotTag string   `json:"SnapshotTag"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		headers := []string{}
		if s.configuration.debug {
			for k, v := range r.Header {
				if len(v) == 0 {
					headers = append(headers, k)
				} else if len(v) == 1 {
					s1 := fmt.Sprintf("%s: %v", k, v[0])
					headers = append(headers, s1)
				} else {
					headers = append(headers, "  "+k+":")
					for _, v2 := range v {
						headers = append(headers, "    "+v2)
					}
				}
			}
		}
		snapshotTag := fmt.Sprintf("%s%s", buildTime, sha1ver)

		versionResponse := response{
			URL:         r.RequestURI,
			Method:      r.Method,
			Headers:     headers,
			Version:     sha1ver,
			BuildTime:   buildTime,
			SnapshotTag: snapshotTag,
		}

		respond(w, r, http.StatusOK, versionResponse)
	}

}
