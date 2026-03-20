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

	{
		v1 := router.Group("/v1")
		v1.POST("/relief", api.UploadHandler)                     // 创建任务
		v1.GET("/relief/download/:jobId", api.DownloadStlHandler) // 下载STL
		v1.GET("/relief/:jobId", api.GetJobHandler)               // 查询任务
		v1.GET("/relief/queue/status", api.QueueStatusHandler)    // 队列状态
		v1.DELETE("/relief/queue/:jobId", api.DeleteJobHandler)   // 删除任务
	}

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	err := router.Run(":9100")
	if err != nil {
		panic(err)
	}
	// router.Run(":3000") for a hard coded port
}
