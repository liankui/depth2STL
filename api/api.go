package api

import (
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

func UploadHandler(c *gin.Context) {
	file, _ := c.FormFile("file")

	jobID := ksuid.New().String()
	inputPath := "/tmp/" + jobID + ".png"
	outputPath := "/tmp/" + jobID + ".stl"

	c.SaveUploadedFile(file, inputPath)

	job := &Job{
		ID:         jobID,
		FilePath:   inputPath,
		OutputPath: outputPath,
		Status:     "queued",
	}

	jobStore.Store(jobID, job)
	jobQueue <- job

	c.JSON(200, gin.H{"jobId": jobID})
}
