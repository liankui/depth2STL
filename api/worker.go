package api

import (
	"image"
	"os"

	depth2 "github.com/chaos-io/depth2STL/depth"
	"github.com/chaos-io/depth2STL/stl"
)

func StartWorkers(n int) {
	for i := 0; i < n; i++ {
		go workerLoop()
	}
}

var semaphore = make(chan struct{}, 4)

func workerLoop() {
	for job := range jobQueue {
		job.Status = "processing"
		semaphore <- struct{}{}

		err := processJob(job)
		<-semaphore
		if err != nil {
			job.Status = "failed"
			continue
		}

		job.Status = "done"
	}
}

func processJob(job *Job) error {
	// 读取图片
	f, err := os.Open(job.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	// 生成深度图
	depth := depth2.GenerateDepthMap4(img, job.DetailLevel, false)

	// 生成 STL
	err = stl.GenerateSTL5(
		depth,
		job.OutputPath,
		job.ModelWidth,
		job.ModelThickness,
		job.BaseThickness,
		job.SubSample,
	)
	if err != nil {
		return err
	}

	return nil
}
