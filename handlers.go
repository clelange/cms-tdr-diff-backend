package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type gitlabCommitList struct {
	ID          string     `json:"id"`
	ShortID     string     `json:"short_id"`
	CreatedAt   *time.Time `json:"created_at"`
	Title       string     `json:"title"`
	AuthorName  string     `json:"author_name"`
	AuthorEmail string     `json:"author_email"`
	Tag         string     `json:"tag"`
}

type triggerStruct struct {
	Project string `uri:"project" binding:"required"`
	Group   string `uri:"group" binding:"required"`
	SHA1    string `uri:"sha1" binding:"required"`
	SHA2    string `uri:"sha2" binding:"required"`
}

func (s *server) handleTypes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, http.StatusOK, projTypes)
	}
}

func (s *server) handleProjects() http.HandlerFunc {
	type response struct {
		Data []gitlabProjectList `json:"data"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		groupID, ok := vars["id"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		log.Println(groupID)
		for key := range groupIDs {
			if key == groupID {
				projectsMu.RLock()
				projects := append([]gitlabProjectList(nil), allProjects[groupID]...)
				projectsMu.RUnlock()
				projectResponse := response{
					Data: projects,
				}
				respond(w, r, http.StatusOK, projectResponse)
				return
			}
		}
		errorMessage := "Project not found: " + groupID
		err := errors.New(errorMessage)
		respondErr(w, r, http.StatusBadRequest, err)
		return
	}
}

func (s *server) handleCommits() http.HandlerFunc {
	type response struct {
		ProjectInfo gitlabProjectList  `json:"project_info"`
		CommitList  []gitlabCommitList `json:"commits"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectGroup, ok := vars["group"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		projectID, ok := vars["id"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		log.Println(projectGroup, projectID)
		projectInfo, _, err := s.getProjectInfo(projectGroup, projectID)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		commitList, err := s.getCommits(projectInfo.ID)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		tagList, err := s.getTags(projectInfo.ID)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		// match tags to commits
		for n, commit := range commitList {
			for _, tag := range tagList {
				if strings.HasPrefix(tag.Name, "CADI-BuildTag") {
					if commit.ShortID == tag.Commit.ShortID {
						commitList[n].Tag = tag.Name
					}
				}
			}
		}
		commitResponse := response{
			ProjectInfo: projectInfo,
			CommitList:  commitList,
		}
		respond(w, r, http.StatusOK, commitResponse)
	}
}

func (s *server) handlePipelineStatus() http.HandlerFunc {
	type response struct {
		JobStatus *diffJobStatus `json:"job_status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		jobID, ok := vars["id"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		log.Println(jobID)
		jobStatus, err := s.getDiffJob(r.Context(), jobID)
		if err != nil {
			if isNotFound(err) {
				respondErr(w, r, http.StatusNotFound, err)
				return
			}
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		pipelineStatusResponse := response{
			JobStatus: jobStatus,
		}
		respond(w, r, http.StatusOK, pipelineStatusResponse)
	}
}

func (s *server) handleJobs() http.HandlerFunc {
	type response struct {
		Jobs []diffJobStatus `json:"jobs"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		jobs, err := s.listDiffJobs(r.Context())
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		respond(w, r, http.StatusOK, response{Jobs: jobs})
	}
}

func (s *server) handleJobDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		jobID, ok := vars["id"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		s.serveJobDownload(w, r, jobID)
	}
}

func (s *server) handleTrigger() http.HandlerFunc {
	type response struct {
		Status string         `json:"status"`
		JobID  string         `json:"job_id"`
		Job    *diffJobStatus `json:"job"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var triggerObject triggerStruct
		if err := decodeBody(r, &triggerObject); err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		jobStatus, err := s.createDiffJob(r.Context(), triggerObject)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		triggerReponse := response{
			Status: "Diff job created successfully!",
			JobID:  jobStatus.ID,
			Job:    jobStatus,
		}
		respond(w, r, http.StatusOK, triggerReponse)
	}
}
