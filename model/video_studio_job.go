package model

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const SystemTaskTypeVideoStudio = "video_studio_jobs"

const (
	StudioJobQueued    = "queued"
	StudioJobRunning   = "running"
	StudioJobSubmitted = "submitted"
	StudioJobSucceeded = "succeeded"
	StudioJobFailed    = "failed"
	StudioJobUncertain = "uncertain"
)

// Input and media live in VIDEO_STUDIO_DATA_DIR, never in a public static tree.
// RequestID is user-scoped so retransmitting an accepted request cannot bill twice.
type StudioJob struct {
	ID          string          `json:"id" gorm:"type:varchar(64);primaryKey"`
	UserID      int             `json:"-" gorm:"uniqueIndex:idx_studio_request;index:idx_studio_history"`
	RequestID   string          `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_studio_request"`
	Kind        string          `json:"kind" gorm:"type:varchar(16)"`
	Operation   string          `json:"operation" gorm:"type:varchar(32)"`
	Group       string          `json:"group" gorm:"type:varchar(64)"`
	Model       string          `json:"model" gorm:"type:varchar(191)"`
	Prompt      string          `json:"prompt" gorm:"type:text"`
	Parameters  json.RawMessage `json:"parameters" gorm:"type:text;serializer:json"`
	Status      string          `json:"status" gorm:"type:varchar(16);index"`
	CreatedAt   int64           `json:"created_at" gorm:"index:idx_studio_history"`
	UpdatedAt   int64           `json:"updated_at"`
	Error       string          `json:"error,omitempty" gorm:"type:text"`
	TaskID      string          `json:"task_id,omitempty" gorm:"type:varchar(64)"`
	ContentType string          `json:"-" gorm:"type:text"`
	Fingerprint string          `json:"-" gorm:"type:varchar(64)"`
	RunnerTask  string          `json:"-" gorm:"type:varchar(64)"`
}

func FindStudioJob(userID int, id string) (*StudioJob, error) {
	var job StudioJob
	err := DB.Where("user_id = ? AND id = ?", userID, id).First(&job).Error
	return &job, err
}

func FindStudioJobRequest(userID int, requestID string) (*StudioJob, error) {
	var job StudioJob
	err := DB.Where("user_id = ? AND request_id = ?", userID, requestID).First(&job).Error
	return &job, err
}

func CreateStudioJob(job *StudioJob) (*StudioJob, bool, error) {
	if err := DB.Create(job).Error; err != nil {
		existing, lookupErr := FindStudioJobRequest(job.UserID, job.RequestID)
		if lookupErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return job, true, nil
}

func ListStudioJobs(userID int, kind string, page *common.PageInfo) ([]StudioJob, int64, error) {
	query := DB.Model(&StudioJob{}).Where("user_id = ?", userID)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	jobs := make([]StudioJob, 0)
	err := query.Order("created_at desc, id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&jobs).Error
	return jobs, total, err
}

func HasPendingStudioJobs() bool {
	var count int64
	return DB.Model(&StudioJob{}).Where("status IN ?", []string{StudioJobQueued, StudioJobRunning, StudioJobSubmitted}).Limit(1).Count(&count).Error == nil && count > 0
}

func UnfinishedStudioJobs() ([]StudioJob, error) {
	var jobs []StudioJob
	err := DB.Where("status IN ?", []string{StudioJobRunning, StudioJobSubmitted}).Order("created_at, id").Find(&jobs).Error
	return jobs, err
}

type StudioJobFilter struct {
	Kind         string
	Group        string
	Model        string
	Models       []string
	ExcludeRoute bool
}

func (f StudioJobFilter) Matches(job StudioJobFilter) bool {
	if f.Kind != job.Kind {
		return false
	}
	if f.Group == "" && f.Model == "" && len(f.Models) == 0 {
		return true
	}
	models := f.Models
	if len(models) == 0 {
		models = []string{f.Model}
	}
	matches := false
	for _, imageModel := range models {
		if imageModel == job.Model {
			matches = f.Group == job.Group
			break
		}
	}
	return matches != f.ExcludeRoute
}

func ClaimStudioJob(runnerTask string, filters ...StudioJobFilter) (*StudioJob, error) {
	var job StudioJob
	query := DB.Where("status = ?", StudioJobQueued)
	if len(filters) > 0 {
		filter := filters[0]
		query = query.Where("kind = ?", filter.Kind)
		if filter.Group != "" || filter.Model != "" || len(filter.Models) > 0 {
			models := filter.Models
			if len(models) == 0 {
				models = []string{filter.Model}
			}
			if filter.ExcludeRoute {
				query = query.Where("("+commonGroupCol+" <> ? OR model NOT IN ?)", filter.Group, models)
			} else {
				query = query.Where(commonGroupCol+" = ? AND model IN ?", filter.Group, models)
			}
		}
	}
	if err := query.Order("created_at, id").First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	now := common.GetTimestamp()
	result := DB.Model(&StudioJob{}).Where("id = ? AND status = ?", job.ID, StudioJobQueued).
		Updates(map[string]any{"status": StudioJobRunning, "runner_task": runnerTask, "updated_at": now})
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, result.Error
	}
	job.Status, job.RunnerTask, job.UpdatedAt = StudioJobRunning, runnerTask, now
	return &job, nil
}

func SaveStudioJobState(job *StudioJob) error {
	job.UpdatedAt = common.GetTimestamp()
	return DB.Model(&StudioJob{}).Where("id = ? AND runner_task = ?", job.ID, job.RunnerTask).
		Where("status <> ?", StudioJobSucceeded).
		Updates(map[string]any{"status": job.Status, "error": job.Error, "task_id": job.TaskID, "updated_at": job.UpdatedAt}).Error
}
