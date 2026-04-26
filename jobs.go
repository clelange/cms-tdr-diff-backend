package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const diffJobLabelComponent = "tdr-diff-job"

var (
	jobIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{6,62}$`)
	projectPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	shaPattern     = regexp.MustCompile(`^[A-Fa-f0-9]{7,40}$`)
)

type diffJobStatus struct {
	ID             string     `json:"id"`
	JobName        string     `json:"job_name"`
	Project        string     `json:"project"`
	Group          string     `json:"group"`
	SHA1           string     `json:"sha1"`
	SHA2           string     `json:"sha2"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	Duration       int64      `json:"duration"`
	ExpiresAt      time.Time  `json:"expires_at"`
	DownloadURL    string     `json:"download_url,omitempty"`
	Artifacts      []artifact `json:"artifacts"`
	Active         int32      `json:"active"`
	Succeeded      int32      `json:"succeeded"`
	Failed         int32      `json:"failed"`
	FailureReason  string     `json:"failure_reason,omitempty"`
	FailureMessage string     `json:"failure_message,omitempty"`
}

type artifact struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

func newKubernetesClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			if home, homeErr := os.UserHomeDir(); homeErr == nil {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}
		if kubeconfig == "" {
			return nil, err
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

func (s *server) createDiffJob(ctx context.Context, trigger triggerStruct) (*diffJobStatus, error) {
	if s.kube == nil {
		return nil, errors.New("kubernetes client unavailable")
	}
	if err := s.validateDiffRequest(trigger); err != nil {
		return nil, err
	}
	if err := s.enforceJobConcurrency(ctx); err != nil {
		return nil, err
	}

	jobID, err := newJobID()
	if err != nil {
		return nil, err
	}
	jobName := jobNameFromID(jobID)
	labels := diffJobLabels(jobID)
	annotations := map[string]string{
		"cms-tdr-diff.cern.ch/project": trigger.Project,
		"cms-tdr-diff.cern.ch/group":   trigger.Group,
		"cms-tdr-diff.cern.ch/sha1":    trigger.SHA1,
		"cms-tdr-diff.cern.ch/sha2":    trigger.SHA2,
	}
	ttl := int32(s.configuration.diffJobTTLSeconds)
	backoffLimit := s.configuration.diffJobBackoffLimit
	activeDeadline := s.configuration.diffJobActiveDeadlineSecs

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Namespace:   s.configuration.namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &activeDeadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: s.configuration.diffJobServiceAccount,
					Containers: []corev1.Container{
						{
							Name:            "tdr-diff",
							Image:           s.configuration.diffImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "JOB_ID", Value: jobID},
								{Name: "REPO_PROJECT", Value: trigger.Project},
								{Name: "REPO_GROUP", Value: trigger.Group},
								{Name: "GIT_SHA1", Value: trigger.SHA1},
								{Name: "GIT_SHA2", Value: trigger.SHA2},
								{Name: "OUTPUT_ROOT", Value: s.configuration.diffOutputMountPath},
								{Name: "WORK_ROOT", Value: s.configuration.diffWorkspaceMountPath},
								{Name: "CI_PROJECT_DIR", Value: "/opt/tdr-diff"},
								{
									Name: "GITLAB_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: s.configuration.diffSecretName},
											Key:                  "GITLAB_TOKEN",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(s.configuration.diffJobCPURequest),
									corev1.ResourceMemory: resource.MustParse(s.configuration.diffJobMemoryRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(s.configuration.diffJobCPULimit),
									corev1.ResourceMemory: resource.MustParse(s.configuration.diffJobMemoryLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "output", MountPath: s.configuration.diffOutputMountPath},
								{Name: "workspace", MountPath: s.configuration.diffWorkspaceMountPath},
								{Name: "tmp", MountPath: "/tmp"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "output",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: s.configuration.diffOutputPVC,
								},
							},
						},
						{
							Name:         "workspace",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name:         "tmp",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					},
				},
			},
		},
	}

	created, err := s.kube.BatchV1().Jobs(s.configuration.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return s.jobStatus(created), nil
}

func (s *server) validateDiffRequest(trigger triggerStruct) error {
	if trigger.Project == "" || trigger.Group == "" || trigger.SHA1 == "" || trigger.SHA2 == "" {
		return errors.New("project, group, sha1, and sha2 are required")
	}
	if !projectPattern.MatchString(trigger.Project) {
		return fmt.Errorf("invalid project name %q", trigger.Project)
	}
	if !shaPattern.MatchString(trigger.SHA1) || !shaPattern.MatchString(trigger.SHA2) {
		return errors.New("sha1 and sha2 must be Git commit hashes")
	}
	for _, group := range s.configuration.groupIds {
		if trigger.Group == group {
			return nil
		}
	}
	return fmt.Errorf("group %q is not configured", trigger.Group)
}

func (s *server) enforceJobConcurrency(ctx context.Context) error {
	if s.configuration.diffMaxConcurrentJobs < 1 {
		return nil
	}
	jobs, err := s.kube.BatchV1().Jobs(s.configuration.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "cms-tdr-diff.cern.ch/component=" + diffJobLabelComponent,
	})
	if err != nil {
		return err
	}
	activeJobs := 0
	for _, job := range jobs.Items {
		if job.Status.Active > 0 {
			activeJobs++
		}
	}
	if activeJobs >= s.configuration.diffMaxConcurrentJobs {
		return fmt.Errorf("too many active diff jobs: %d active, limit is %d", activeJobs, s.configuration.diffMaxConcurrentJobs)
	}
	return nil
}

func (s *server) listDiffJobs(ctx context.Context) ([]diffJobStatus, error) {
	if s.kube == nil {
		return nil, errors.New("kubernetes client unavailable")
	}
	jobs, err := s.kube.BatchV1().Jobs(s.configuration.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "cms-tdr-diff.cern.ch/component=" + diffJobLabelComponent,
	})
	if err != nil {
		return nil, err
	}
	statuses := make([]diffJobStatus, 0, len(jobs.Items))
	for i := range jobs.Items {
		statuses = append(statuses, *s.jobStatus(&jobs.Items[i]))
	}
	return statuses, nil
}

func (s *server) getDiffJob(ctx context.Context, id string) (*diffJobStatus, error) {
	if s.kube == nil {
		return nil, errors.New("kubernetes client unavailable")
	}
	if !jobIDPattern.MatchString(id) {
		return nil, fmt.Errorf("invalid job id %q", id)
	}
	job, err := s.kube.BatchV1().Jobs(s.configuration.namespace).Get(ctx, jobNameFromID(id), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return s.jobStatus(job), nil
}

func (s *server) jobStatus(job *batchv1.Job) *diffJobStatus {
	id := strings.TrimPrefix(job.Name, "tdr-diff-")
	createdAt := job.CreationTimestamp.Time
	status := "pending"
	if job.Status.Active > 0 {
		status = "running"
	}
	if job.Status.Succeeded > 0 {
		status = "success"
	}
	if job.Status.Failed > 0 {
		status = "failed"
	}

	var startedAt *time.Time
	if job.Status.StartTime != nil {
		start := job.Status.StartTime.Time
		startedAt = &start
	}
	var finishedAt *time.Time
	if job.Status.CompletionTime != nil {
		finish := job.Status.CompletionTime.Time
		finishedAt = &finish
	}

	duration := int64(0)
	if startedAt != nil {
		end := time.Now()
		if finishedAt != nil {
			end = *finishedAt
		}
		duration = int64(end.Sub(*startedAt).Seconds())
	}

	var failureReason, failureMessage string
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			failureReason = condition.Reason
			failureMessage = condition.Message
			break
		}
	}

	artifacts := []artifact{}
	zipPath := s.outputZipPath(id)
	if _, err := os.Stat(zipPath); err == nil {
		artifacts = append(artifacts, artifact{
			Filename: filepath.Base(zipPath),
			URL:      "/jobs/" + id + "/download",
		})
	}

	return &diffJobStatus{
		ID:             id,
		JobName:        job.Name,
		Project:        job.Annotations["cms-tdr-diff.cern.ch/project"],
		Group:          job.Annotations["cms-tdr-diff.cern.ch/group"],
		SHA1:           job.Annotations["cms-tdr-diff.cern.ch/sha1"],
		SHA2:           job.Annotations["cms-tdr-diff.cern.ch/sha2"],
		Status:         status,
		CreatedAt:      createdAt,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		Duration:       duration,
		ExpiresAt:      createdAt.Add(time.Duration(s.configuration.diffOutputRetentionDays) * 24 * time.Hour),
		DownloadURL:    "/jobs/" + id + "/download",
		Artifacts:      artifacts,
		Active:         job.Status.Active,
		Succeeded:      job.Status.Succeeded,
		Failed:         job.Status.Failed,
		FailureReason:  failureReason,
		FailureMessage: failureMessage,
	}
}

func (s *server) outputZipPath(id string) string {
	return filepath.Join(s.configuration.diffOutputPath, id, "output.zip")
}

func (s *server) serveJobDownload(w http.ResponseWriter, r *http.Request, id string) {
	if !jobIDPattern.MatchString(id) {
		respondErr(w, r, http.StatusBadRequest, "invalid job id")
		return
	}
	zipPath := s.outputZipPath(id)
	if _, err := os.Stat(zipPath); err != nil {
		if os.IsNotExist(err) {
			respondErr(w, r, http.StatusNotFound, "job output is not available")
			return
		}
		log.Printf("stat output zip failed: %v", err)
		respondErr(w, r, http.StatusInternalServerError, "could not read job output")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-output.zip"`, id))
	http.ServeFile(w, r, zipPath)
}

func newJobID() (string, error) {
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(randomBytes)), nil
}

func jobNameFromID(id string) string {
	return "tdr-diff-" + id
}

func diffJobLabels(id string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":         "cms-tdr-diff",
		"app.kubernetes.io/component":    "diff-job",
		"cms-tdr-diff.cern.ch/component": diffJobLabelComponent,
		"cms-tdr-diff.cern.ch/job-id":    id,
	}
}

func isNotFound(err error) bool {
	return apierrors.IsNotFound(err)
}
