package main

import (
	"github.com/gorilla/mux"
)

func (s *server) newRouter(apiToken string, frontendOrigin string) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/ping", s.handlePing())
	r.HandleFunc("/lastUpdated", s.handleLastUpdated())
	r.HandleFunc("/version", s.handleVersion())
	r.HandleFunc("/types", withCORS(withAPIKey(s.handleTypes(), apiToken), frontendOrigin))
	r.HandleFunc("/projects/{id}", withCORS(withAPIKey(s.handleProjects(), apiToken), frontendOrigin))
	r.HandleFunc("/commits/{group}/{id}", withCORS(withAPIKey(s.handleCommits(), apiToken), frontendOrigin))
	r.HandleFunc("/status/pipeline/{id}", withCORS(withAPIKey(s.handlePipelineStatus(), apiToken), frontendOrigin))
	r.HandleFunc("/trigger", withCORS(withAPIKey(s.handleTrigger(), apiToken), frontendOrigin)).Methods("POST")
	return r
}
