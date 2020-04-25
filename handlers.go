package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/xanzy/go-gitlab"
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
				projectResponse := response{
					Data: allProjects[groupID],
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
		JobStatus *gitlab.Job `json:"job_status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		pipelineIDString, ok := vars["id"]
		if !ok {
			respondErr(w, r, http.StatusBadRequest, ok)
			return
		}
		log.Println(pipelineIDString)
		pipelineID, err := strconv.Atoi(pipelineIDString)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		pipelineJobs, _, err := s.gl.Jobs.ListPipelineJobs(pipelineProjectID, pipelineID, nil)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		jobID := pipelineJobs[0].ID
		job, _, err := s.gl.Jobs.GetJob(pipelineProjectID, jobID, nil)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		pipelineStatusResponse := response{
			JobStatus: job,
		}
		respond(w, r, http.StatusOK, pipelineStatusResponse)
	}
}

func (s *server) handleTrigger() http.HandlerFunc {
	type response struct {
		Status     string `json:"status"`
		PipelineID int    `json:"pipeline_id"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var triggerObject triggerStruct
		if err := decodeBody(r, &triggerObject); err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		var variables = make(map[string]string)
		variables["REPO_PROJECT"] = triggerObject.Project
		variables["REPO_GROUP"] = triggerObject.Group
		variables["GIT_SHA1"] = triggerObject.SHA1
		variables["GIT_SHA2"] = triggerObject.SHA2

		referenceBranch := "master"
		pipelineOptions := &gitlab.RunPipelineTriggerOptions{
			Ref:       &referenceBranch,
			Token:     &s.configuration.triggerToken,
			Variables: variables,
		}

		pipeline, _, err := s.gl.PipelineTriggers.RunPipelineTrigger(pipelineProjectID, pipelineOptions)
		if err != nil {
			respondErr(w, r, http.StatusBadRequest, err)
			return
		}
		triggerReponse := response{
			Status:     "Pipeline triggered successfully!",
			PipelineID: pipeline.ID,
		}
		respond(w, r, http.StatusOK, triggerReponse)
	}
}
