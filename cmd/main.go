package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/chaos-io/depth2STL/api"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

func main() {
	// Creates a gin router with default middleware:
	// logger and recovery (crash-free) middleware
	router := gin.Default()
	router.MaxMultipartMemory = 10 << 20 // 10 MiB

	{
		v1 := router.Group("/v1")
		v1.POST("/relief", api.CreateHandler)                             // 创建任务
		v1.GET("/relief/download/image/:jobId", api.DownloadImageHandler) // 下载image
		v1.GET("/relief/download/stl/:jobId", api.DownloadStlHandler)     // 下载STL
		v1.GET("/relief/:jobId", api.GetJobHandler)                       // 查询任务
		v1.GET("/relief/queue/status", api.QueueStatusHandler)            // 队列状态
		v1.DELETE("/relief/queue/:jobId", api.DeleteJobHandler)           // 删除任务
	}

	router.GET("/config.js", frontendConfigHandler)
	router.Static("/frontend", "./frontend")
	router.StaticFile("/", "./frontend/index.html")

	crontab()

	port := os.Getenv("PORT")
	if port == "" {
		port = "31101"
	}

	err := router.Run(":" + port)
	if err != nil {
		panic(err)
	}
	// router.Run(":3000") for a hard coded port
}

func frontendConfigHandler(c *gin.Context) {
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "31101"
		}

		apiBaseURL = fmt.Sprintf("%s://%s:%s/v1", requestScheme(c), requestHost(c), port)
	}

	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.String(200, "window.APP_CONFIG = { apiBaseUrl: %q };\n", strings.TrimRight(apiBaseURL, "/"))
}

func requestScheme(c *gin.Context) string {
	if c.GetHeader("X-Forwarded-Proto") == "https" {
		return "https"
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(c *gin.Context) string {
	host := c.Request.Host
	if host == "" {
		return "localhost"
	}

	if name, _, err := net.SplitHostPort(host); err == nil {
		return name
	}

	return host
}

func crontab() {
	// 创建 cron 实例（支持秒级）
	c := cron.New(cron.WithSeconds())

	// 每一小时执行
	_, err := c.AddFunc("@hourly", func() {
		// _, err := c.AddFunc("* */5 * * * *", func() { // debug
		api.ClearJobs()
	})
	if err != nil {
		panic(err)
	}

	c.Start()
}
