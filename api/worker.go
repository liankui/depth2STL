package api

import (
	"image"
	"os"
	"sync/atomic"
	"time"

	"github.com/chaos-io/depth2STL/depth"
	"github.com/chaos-io/depth2STL/stl"
)

func init() {
	StartWorkers(2)
}

func StartWorkers(n int) {
	for i := 0; i < n; i++ {
		go workerLoop()
	}
}

func workerLoop() {
	for job := range jobQueue {
		atomic.AddInt32(&activeWorkers, 1)
		job.Status = StatusProcessing
		job.CreatedAt = time.Now()

		// err := processJob(job)
		// if err != nil {
		// 	job.Status = StatusFailed
		// 	job.Error = err.Error()
		// } else {
		time.Sleep(20 * time.Second)
		job.Status = StatusDone
		// }

		job.UpdatedAt = time.Now()
		atomic.AddInt32(&activeWorkers, -1)
	}
}

func processJob(job *Job) error {
	// 读取图片
	f, err := os.Open(job.FilePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	// 生成深度图
	_depth := depth.GenerateDepthMap4(img, job.DetailLevel, false)

	// 生成 STL
	err = stl.GenerateSTL5(_depth, job.OutputPath, job.ModelWidth, job.ModelThickness, job.BaseThickness, job.SubSample)
	if err != nil {
		return err
	}

	return nil
}
