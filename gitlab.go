package main

import (
	"errors"
	"log"
	"time"

	"github.com/xanzy/go-gitlab"
)

type gitlabProjectList struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	WebURL         string     `json:"web_url"`
	LastActivityAt *time.Time `json:"last_activity_at"`
	Description    string     `json:"description"`
}

func (s *server) getProjectInfo(projectGroup string, projectID string) (gitlabProjectList, *gitlab.Response, error) {
	projectPath := "tdr/" + projectGroup + "/" + projectID
	project, response, err := s.gl.Projects.GetProject(projectPath, nil)
	if err != nil {
		return gitlabProjectList{}, response, err
	}
	projectInfo := gitlabProjectList{
		ID:             project.ID,
		Name:           project.Name,
		Description:    project.Description,
		WebURL:         project.WebURL,
		LastActivityAt: project.LastActivityAt,
	}
	return projectInfo, response, err
}

func (s *server) getCommits(projectID int) ([]gitlabCommitList, error) {
	maxPages := 100
	currentPage := 1
	var commitList []gitlabCommitList
	for currentPage <= maxPages {
		var listQueryOptions = &gitlab.ListCommitsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100, // this is the maximum one can ask for
				Page:    currentPage,
			}}
		commits, response, err := s.gl.Commits.ListCommits(projectID, listQueryOptions)
		if err != nil {
			log.Print(err)
			return commitList, err
		}
		currentCommitList := make([]gitlabCommitList, len(commits))
		for i := 0; i < len(commits); i++ {
			currentCommitList[i] = gitlabCommitList{
				ID:          commits[i].ID,
				ShortID:     commits[i].ShortID,
				CreatedAt:   commits[i].CreatedAt,
				Title:       commits[i].Title,
				AuthorName:  commits[i].AuthorName,
				AuthorEmail: commits[i].AuthorEmail,
				Tag:         string(""),
			}
			commitList = append(commitList, currentCommitList[i])
		}
		maxPages = response.TotalPages
		currentPage++
	}
	log.Println("Number of commits:", len(commitList))
	return commitList, nil
}

func (s *server) getTags(projectID int) ([]*gitlab.Tag, error) {

	// var tagList []gitlabTagList
	var listQueryOptions = &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{}}
	tags, _, err := s.gl.Tags.ListTags(projectID, listQueryOptions)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	return tags, err

}

// check that provided subgroups exist in project
func validateSubgroups(groupID int, gl *gitlab.Client, configuration Configuration) (map[string]int, error) {
	groups, _, err := gl.Groups.ListSubgroups(groupID, nil)
	if err != nil {
		log.Print(err)
	}

	groupIDMap := make(map[string]int)
	for _, n := range configuration.groupIds {
		found := false
		for _, group := range groups {
			if n == group.Name {
				groupIDMap[group.Name] = group.ID
				found = true
			}
		}
		if !found {
			err = errors.New("subgroup name not found")
			return groupIDMap, err
		}
	}
	return groupIDMap, err
}

// get all projects for a given subgroup
func getProjects(groupID int, gl *gitlab.Client) ([]gitlabProjectList, error) {
	maxPages := 100
	currentPage := 1
	var projectList []gitlabProjectList
	for currentPage <= maxPages {
		var listQueryOptions = &gitlab.ListGroupProjectsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100, // this is the maximum one can ask for
				Page:    currentPage,
			}}
		projects, response, err := gl.Groups.ListGroupProjects(groupID, listQueryOptions)
		if err != nil {
			log.Print(err)
			return projectList, err
		}
		currentProjectList := make([]gitlabProjectList, len(projects))
		for i := 0; i < len(projects); i++ {
			currentProjectList[i] = gitlabProjectList{
				ID:             projects[i].ID,
				Name:           projects[i].Name,
				WebURL:         projects[i].WebURL,
				LastActivityAt: projects[i].LastActivityAt,
				Description:    projects[i].Description,
			}
			projectList = append(projectList, currentProjectList[i])
		}
		maxPages = response.TotalPages
		currentPage++
	}
	log.Println("Number of projects:", len(projectList))
	return projectList, nil
}

func updateProjects(groupIDs map[string]int, gl *gitlab.Client) (map[string][]gitlabProjectList, error) {
	allProjects := make(map[string][]gitlabProjectList)
	var err error
	for value, key := range groupIDs {
		log.Println("Getting projects for group:", value, key)
		allProjects[value], err = getProjects(key, gl)
		if err != nil {
			log.Print(err)
		}
	}
	return allProjects, err
}
