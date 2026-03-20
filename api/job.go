package api

import "sync"

type Job struct {
	ID             string
	FilePath       string
	OutputPath     string
	DetailLevel    float64
	ModelWidth     float64
	ModelThickness float64
	BaseThickness  float64
	SubSample      int
	Status         string
}

var jobQueue = make(chan *Job, 1000)
var jobStore sync.Map // jobId -> *Job
