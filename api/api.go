package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

func UploadHandler(c *gin.Context) {
	file, _ := c.FormFile("file")

	ext := filepath.Ext(file.Filename)
	filename := strings.TrimSuffix(file.Filename, ext)
	jobID := filename + "_" + ksuid.New().String()
	pwd, _ := os.Getwd()
	tmpDir := filepath.Join(pwd, "tmp")
	_ = os.MkdirAll(tmpDir, os.ModePerm)

	inputPath := filepath.Join(tmpDir, jobID+".png")
	outputPath := filepath.Join(tmpDir, jobID+".stl")

	err := c.SaveUploadedFile(file, inputPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	job := &Job{
		ID:         jobID,
		Name:       filename,
		FilePath:   inputPath,
		OutputPath: outputPath,
		Status:     StatusQueued,
	}

	jobStore.Store(jobID, job)
	jobQueue <- job

	c.JSON(200, gin.H{"jobId": jobID})
}

func DownloadStlHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	val, ok := jobStore.Load(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	job := val.(*Job)

	if job.Status != StatusDone {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "file not ready",
			"status": job.Status,
		})
		return
	}

	// 文件是否存在
	if _, err := os.Stat(job.OutputPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file missing"})
		return
	}

	// 设置下载头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.stl", job.ID))
	c.Header("Content-Transfer-Encoding", "binary")

	c.File(job.OutputPath)
}

func GetJobHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	val, ok := jobStore.Load(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	job := val.(*Job)

	resp := gin.H{
		"jobId":  job.ID,
		"status": job.Status,
	}

	if job.Status == StatusDone {
		resp["downloadUrl"] = fmt.Sprintf("/download/%s", job.ID)
	}

	if job.Status == StatusFailed {
		resp["error"] = job.Error
	}

	c.JSON(http.StatusOK, resp)
}

func QueueStatusHandler(c *gin.Context) {
	var (
		total      int
		queued     int
		processing int
		done       int
		failed     int
	)

	jobStore.Range(func(_, value any) bool {
		job := value.(*Job)
		total++

		switch job.Status {
		case StatusQueued:
			queued++
		case StatusProcessing:
			processing++
		case StatusDone:
			done++
		case StatusFailed:
			failed++
		}
		return true
	})

	c.JSON(http.StatusOK, gin.H{
		"totalJobs": total,

		"queue": gin.H{
			"queued":     queued,
			"processing": processing,
			"done":       done,
			"failed":     failed,
		},

		"runtime": gin.H{
			"queueLength":   len(jobQueue),
			"activeWorkers": atomic.LoadInt32(&activeWorkers),
			"maxQueueSize":  cap(jobQueue),
		},
	})
}

func DeleteJobHandler(c *gin.Context) {
	jobID := c.Param("jobId")

	val, ok := jobStore.Load(jobID)
	if !ok {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	job := val.(*Job)

	_ = os.Remove(job.FilePath)
	_ = os.Remove(job.OutputPath)

	jobStore.Delete(jobID)

	c.JSON(200, gin.H{"message": "deleted"})
}
