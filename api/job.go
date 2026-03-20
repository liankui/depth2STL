package api

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

var (
	// TODO: refactor by redis
	jobQueue = make(chan *Job, 10)
	jobStore sync.Map // map[string]*Job

	activeWorkers int32 // 当前正在处理任务数（原子计数）
)

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusDone       JobStatus = "done"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID             string
	Name           string
	FilePath       string
	OutputPath     string
	DetailLevel    float64
	ModelWidth     float64
	ModelThickness float64
	BaseThickness  float64
	SubSample      int
	Status         JobStatus
	Error          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func ClearJobs() {
	oneDayAgo := time.Now().AddDate(0, 0, -1)

	jobStore.Range(func(_, value any) bool {
		job := value.(*Job)

		// 清理1天前的任务
		if job.CreatedAt.After(oneDayAgo) {
			slog.Info("clear job", "id", job.ID)
			_ = os.Remove(job.FilePath)
			_ = os.Remove(job.OutputPath)
			jobStore.Delete(job.ID)
		}

		return true
	})

}
