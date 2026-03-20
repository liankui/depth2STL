package api

import (
	"sync"
	"time"
)

var (
	jobQueue = make(chan *Job, 1000)
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
