package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"k8s.io/client-go/kubernetes"
)

var (
	sha1ver   string // sha1 revision used to build the program
	buildTime string // when the executable was built
)

var (
	flgVersion bool // for flag parsing
)

type tdrTypes struct {
	Names []string `json:"names"`
}

var (
	lastUpdated time.Time                      // when projects have last been updated
	allProjects map[string][]gitlabProjectList // all GitLab projects
	projTypes   *tdrTypes                      // all types available in tdr repository
	groupIDs    map[string]int64               // all available group IDs
	projectsMu  sync.RWMutex
)

func parseCmdLineFlags() {
	flag.BoolVar(&flgVersion, "version", false, "if true, print version and exit")
	flag.Parse()
	if flgVersion {
		fmt.Printf("Build snapshot tag %s%s\n", buildTime, sha1ver)
		os.Exit(0)
	}
}

type server struct {
	gl            *gitlab.Client
	kube          kubernetes.Interface
	configuration *Configuration
}

func main() {

	parseCmdLineFlags()
	log.Printf("Build snapshot tag %s%s\n", buildTime, sha1ver)

	v1, err := readConfig()
	if err != nil {
		log.Panicln("Configuration error", err)
	}

	configuration, err := validateAndSetConfig(v1)
	if err != nil {
		log.Panicln(err)
	}

	gl, err := gitlab.NewClient(configuration.gitlabToken, gitlab.WithBaseURL(configuration.gitlabURL))
	if err != nil {
		log.Panicln(err)
	}

	s := &server{
		gl:            gl,
		configuration: &configuration,
	}
	kube, err := newKubernetesClient()
	if err != nil {
		log.Printf("Kubernetes client unavailable: %v", err)
	} else {
		s.kube = kube
	}

	groupIDs, err = validateSubgroups(16284, gl, configuration) // this is the tdr group
	if err != nil {
		log.Print(err)
	}
	log.Println("Group IDs:", groupIDs)

	allProjects = make(map[string][]gitlabProjectList)
	refreshProjects := func() {
		log.Println("updating projects...")
		tempAllProjects, err := updateProjects(groupIDs, gl)
		if err != nil {
			log.Println("Updating projects failed", err)
			return
		}
		projectsMu.Lock()
		lastUpdated = time.Now()
		allProjects = tempAllProjects
		projectsMu.Unlock()
		log.Println("Done updating at", lastUpdated)
	}

	types := make([]string, 0, len(configuration.groupIds))
	for _, key := range configuration.groupIds {
		types = append(types, key)
	}
	log.Println("Types:", types)
	projTypes = &tdrTypes{
		Names: types,
	}

	r := s.newRouter(configuration.apiToken, configuration.frontendOrigin)

	srv := &http.Server{
		Addr: s.configuration.address,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	log.Println("Starting web server on", configuration.address)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Print(err)
		}
	}()

	ticker := time.NewTicker(time.Duration(configuration.updateIntervalSeconds) * time.Second)
	defer ticker.Stop()
	go func() {
		refreshProjects()
		for range ticker.C {
			refreshProjects()
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT or SIGTERM.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block until we receive our signal.
	<-c

	log.Println("Stopping...")

	// TODO: implement callback from GitLab for status update
	// TODO: Get only commits of last N days
	// TODO: Improve error messages returned
	// TODO: Implement better logging making use of DEBUG flag

}
