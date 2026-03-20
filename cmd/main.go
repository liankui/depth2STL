package main

import (
	"github.com/chaos-io/depth2STL/api"
	"github.com/gin-gonic/gin"
)

func main() {
	// Creates a gin router with default middleware:
	// logger and recovery (crash-free) middleware
	router := gin.Default()
	router.MaxMultipartMemory = 10 << 20 // 10 MiB

	router.GET("/v1/relief/{jobId}", getting)
	router.POST("/v1/relief", api.UploadHandler)

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	router.Run()
	// router.Run(":3000") for a hard coded port
}
