package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Configuration structure
type Configuration struct {
	address                   string
	frontendOrigin            string
	gitlabToken               string
	apiToken                  string
	gitlabURL                 string
	debug                     bool
	groupIds                  []string
	commitHistoryDays         int
	updateIntervalSeconds     int
	namespace                 string
	diffImage                 string
	diffOutputPVC             string
	diffOutputPath            string
	diffOutputMountPath       string
	diffWorkspaceMountPath    string
	diffSecretName            string
	diffJobServiceAccount     string
	diffJobTTLSeconds         int
	diffJobActiveDeadlineSecs int64
	diffJobBackoffLimit       int32
	diffJobCPURequest         string
	diffJobCPULimit           string
	diffJobMemoryRequest      string
	diffJobMemoryLimit        string
	diffOutputRetentionDays   int
	diffMaxConcurrentJobs     int
}

func readConfig() (*viper.Viper, error) {
	v := viper.New()
	// Use envrironment variables for hosted version with VIPER_ prefix
	v.SetEnvPrefix("viper") // will be uppercased automatically
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// v.BindEnv("gitlabToken")
	// v.BindEnv("triggerToken")
	v.SetDefault("address", ":8000")
	v.SetDefault("frontendOrigin", "http://localhost:3000")
	v.SetDefault("gitlabURL", "https://gitlab.cern.ch/api/v4")
	v.SetDefault("debug", false)
	v.SetDefault("commitHistoryDays", 90)
	v.SetDefault("updateIntervalSeconds", 600)
	v.SetDefault("groupIds", []string{
		"papers", "notes",
	})
	v.SetDefault("namespace", defaultNamespace())
	v.SetDefault("diffImage", "image-registry.openshift-image-registry.svc:5000/cms-tdr-diff-dev/tdr-diff:latest")
	v.SetDefault("diffOutputPVC", "tdr-diff-output")
	v.SetDefault("diffOutputPath", "/data/diffs")
	v.SetDefault("diffOutputMountPath", "/outputs")
	v.SetDefault("diffWorkspaceMountPath", "/workspace")
	v.SetDefault("diffSecretName", "cms-tdr-diff-secrets")
	v.SetDefault("diffJobServiceAccount", "tdr-diff-runner")
	v.SetDefault("diffJobTTLSeconds", 864000)
	v.SetDefault("diffJobActiveDeadlineSecs", 21600)
	v.SetDefault("diffJobBackoffLimit", 0)
	v.SetDefault("diffJobCPURequest", "500m")
	v.SetDefault("diffJobCPULimit", "1")
	v.SetDefault("diffJobMemoryRequest", "1Gi")
	v.SetDefault("diffJobMemoryLimit", "2Gi")
	v.SetDefault("diffOutputRetentionDays", 10)
	v.SetDefault("diffMaxConcurrentJobs", 2)
	v.SetConfigName("config")
	v.AddConfigPath("config")
	v.AutomaticEnv()
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Println("Config file not found. Trying to use environment variables.")
			return v, nil
		}
	}
	return v, err
}

func defaultNamespace() string {
	if namespace := os.Getenv("POD_NAMESPACE"); namespace != "" {
		return namespace
	}
	if namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(namespaceBytes))
	}
	return "cms-tdr-diff-dev"
}

func validateAndSetConfig(v1 *viper.Viper) (Configuration, error) {
	var configuration Configuration
	configuration.address = v1.GetString("address")
	configuration.frontendOrigin = v1.GetString("frontendOrigin")
	configuration.gitlabURL = v1.GetString("gitlabURL")
	configuration.debug = v1.GetBool("debug")
	configuration.groupIds = stringList(v1.Get("groupIds"))
	configuration.commitHistoryDays = v1.GetInt("commitHistoryDays")
	configuration.updateIntervalSeconds = v1.GetInt("updateIntervalSeconds")
	configuration.gitlabToken = v1.GetString("gitlabToken")
	configuration.namespace = v1.GetString("namespace")
	configuration.diffImage = v1.GetString("diffImage")
	configuration.diffOutputPVC = v1.GetString("diffOutputPVC")
	configuration.diffOutputPath = v1.GetString("diffOutputPath")
	configuration.diffOutputMountPath = v1.GetString("diffOutputMountPath")
	configuration.diffWorkspaceMountPath = v1.GetString("diffWorkspaceMountPath")
	configuration.diffSecretName = v1.GetString("diffSecretName")
	configuration.diffJobServiceAccount = v1.GetString("diffJobServiceAccount")
	configuration.diffJobTTLSeconds = v1.GetInt("diffJobTTLSeconds")
	configuration.diffJobActiveDeadlineSecs = v1.GetInt64("diffJobActiveDeadlineSecs")
	configuration.diffJobBackoffLimit = int32(v1.GetInt("diffJobBackoffLimit"))
	configuration.diffJobCPURequest = v1.GetString("diffJobCPURequest")
	configuration.diffJobCPULimit = v1.GetString("diffJobCPULimit")
	configuration.diffJobMemoryRequest = v1.GetString("diffJobMemoryRequest")
	configuration.diffJobMemoryLimit = v1.GetString("diffJobMemoryLimit")
	configuration.diffOutputRetentionDays = v1.GetInt("diffOutputRetentionDays")
	configuration.diffMaxConcurrentJobs = v1.GetInt("diffMaxConcurrentJobs")

	if configuration.gitlabToken == "" {
		errorMessage := "gitlabToken cannot be empty."
		err := errors.New(errorMessage)
		return configuration, err
	}

	configuration.apiToken = v1.GetString("apiToken")
	if configuration.apiToken == "" {
		errorMessage := "apiToken cannot be empty."
		err := errors.New(errorMessage)
		return configuration, err
	}

	fmt.Printf("Reading config for address = %s\n", configuration.address)
	fmt.Printf("Reading config for frontendOrigin = %s\n", configuration.frontendOrigin)
	fmt.Printf("Reading config for gitlabURL = %s\n", configuration.gitlabURL)
	fmt.Printf("Reading config for debug = %t\n", configuration.debug)
	fmt.Printf("Reading config for groupIds = %#v\n", configuration.groupIds)
	fmt.Printf("Reading config for commitHistoryDays = %d\n", configuration.commitHistoryDays)
	fmt.Printf("Reading config for updateIntervalSeconds = %d\n", configuration.updateIntervalSeconds)
	fmt.Printf("Reading config for namespace = %s\n", configuration.namespace)
	fmt.Printf("Reading config for diffImage = %s\n", configuration.diffImage)
	fmt.Printf("Reading config for diffOutputPVC = %s\n", configuration.diffOutputPVC)
	fmt.Printf("Reading config for diffOutputPath = %s\n", configuration.diffOutputPath)
	fmt.Printf("Reading config for diffOutputRetentionDays = %d\n", configuration.diffOutputRetentionDays)
	fmt.Printf("Reading config for diffMaxConcurrentJobs = %d\n", configuration.diffMaxConcurrentJobs)
	// fmt.Printf("Reading config for gitlabToken = %s\n", configuration.gitlabToken)
	// fmt.Printf("Reading config for apiToken = %s\n", configuration.apiToken)
	return configuration, nil
}

func stringList(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case string:
		return strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\t'
		})
	default:
		return nil
	}
}
