package api

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
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

		err := processJob(job)
		if err != nil {
			fmt.Printf("process job err: %v\n", err)
			job.Status = StatusFailed
			job.Error = err.Error()
		} else {
			// time.Sleep(20 * time.Second)
			job.Status = StatusDone
		}

		job.UpdatedAt = time.Now()
		atomic.AddInt32(&activeWorkers, -1)
	}
}

func processJob(job *Job) error {
	fmt.Printf("processing jobId:%s\n", job.ID)
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
	var gray *image.Gray
	if job.SkipDepth {
		gray = depth.ConvertToGray(img)
	} else {
		gray = depth.GenerateDepthMap4(img, job.Invert)
	}
	f, err = os.Create(job.GrayImgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	err = png.Encode(f, gray)
	if err != nil {
		return err
	}
	fmt.Printf("gen img, path:%s\n", job.GrayImgPath)

	// 生成 STL
	err = stl.GenerateSTL5(gray, job.StlPath, job.ModelWidth, job.ModelThickness, job.BaseThickness, job.SubSample)
	if err != nil {
		return err
	}
	fmt.Printf("gen stl, path:%s\n", job.StlPath)

	return nil
}
