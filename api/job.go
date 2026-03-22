package api

import (
	"fmt"
	"os"
	"path"
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
	ImagePath      string
	StlPath        string
	ModelWidth     float64 // 模型宽度（毫米，默认：50.0）
	ModelThickness float64 // 模型最大高度（毫米，默认：5.0）
	BaseThickness  float64 // 底座高度（毫米，默认：2.0）
	SkipConv       bool    // 跳过深度图处理（默认：false）
	Invert         bool    // 反转浮雕（默认：false）
	SubSample      int     // 精度 1:普通 2:推荐（质量高4倍） 3:高精度
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
		if job.CreatedAt.Before(oneDayAgo) {
			// if job.CreatedAt.After(oneDayAgo) { // debug
			// 	if job.Status == StatusQueued || job.Status == StatusProcessing {
			// 		return true
			// 	}

			dir := path.Dir(job.FilePath)
			fmt.Printf("clear job, id:%s, path:%s\n", job.ID, dir)
			err := os.RemoveAll(dir)
			if err != nil {
				fmt.Printf("clear job error: %s\n", err)
			}
			jobStore.Delete(job.ID)
		}

		return true
	})

}
