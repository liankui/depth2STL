package api

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

var pwd, _ = os.Getwd()

func parseFloat64Form(c *gin.Context, key string, defaultValue float64) (float64, error) {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return defaultValue, nil
	}

	return strconv.ParseFloat(value, 64)
}

func parseIntForm(c *gin.Context, key string, defaultValue int) (int, error) {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return defaultValue, nil
	}

	return strconv.Atoi(value)
}

func parseIntFormWithAliases(c *gin.Context, defaultValue int, keys ...string) (int, error) {
	for _, key := range keys {
		value := strings.TrimSpace(c.PostForm(key))
		if value == "" {
			continue
		}

		return strconv.Atoi(value)
	}

	return defaultValue, nil
}

func parseBoolForm(c *gin.Context, key string, defaultValue bool) (bool, error) {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return defaultValue, nil
	}

	return strconv.ParseBool(value)
}

func CreateHandler(c *gin.Context) {
	modelWidth, err := parseFloat64Form(c, "modelWidth", 50.0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid modelWidth"})
		return
	}

	modelThickness, err := parseFloat64Form(c, "modelThickness", 5.0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid modelThickness"})
		return
	}

	baseThickness, err := parseFloat64Form(c, "baseThickness", 2.0)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid baseThickness"})
		return
	}

	skipConv, err := parseBoolForm(c, "skipConv", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skipConv"})
		return
	}

	invert, err := parseBoolForm(c, "invert", false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invert"})
		return
	}

	detailLevel, err := parseIntFormWithAliases(c, 1, "detailLevel", "subSample")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid detailLevel"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := strings.TrimSuffix(file.Filename, ext)

	jobID := ksuid.New().String()
	tmpDir := filepath.Join(pwd, "tmp", jobID)
	_ = os.MkdirAll(tmpDir, os.ModePerm)

	inputPath := filepath.Join(tmpDir, file.Filename)
	imgPath := filepath.Join(tmpDir, jobID+ext)
	stlPath := filepath.Join(tmpDir, jobID+".stl")

	err = c.SaveUploadedFile(file, inputPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	job := &Job{
		ID:             jobID,
		Name:           filename,
		FilePath:       inputPath,
		ImagePath:      imgPath,
		StlPath:        stlPath,
		ModelWidth:     modelWidth,
		ModelThickness: modelThickness,
		BaseThickness:  baseThickness,
		SkipConv:       skipConv,
		Invert:         invert,
		DetailLevel:    detailLevel,
		Status:         StatusQueued,
	}

	select {
	case jobQueue <- job:
		jobStore.Store(jobID, job)
	case <-time.After(3 * time.Second):
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "job queue is busy, try again later",
		})
		return
	}

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
	if _, err := os.Stat(job.StlPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file missing"})
		return
	}

	// 设置下载头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.stl", job.ID))
	c.Header("Content-Transfer-Encoding", "binary")

	c.File(job.StlPath)
}

func DownloadImageHandler(c *gin.Context) {
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
	if _, err := os.Stat(job.ImagePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file missing"})
		return
	}

	// 设置下载头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.png", job.ID))
	c.Header("Content-Transfer-Encoding", "binary")

	c.File(job.ImagePath)
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

	_ = os.Remove(path.Dir(job.FilePath))

	jobStore.Delete(jobID)

	c.JSON(200, gin.H{"message": "deleted"})
}
