package service

import (
	"log"
	"time"

	"daidai-panel/database"
	"daidai-panel/model"
)

var globalScheduler *SchedulerV2
var globalExecutor *TaskExecutor

func InitSchedulerV2() {
	globalExecutor = NewTaskExecutor()

	workerCount := model.GetRegisteredConfigInt("max_concurrent_tasks")
	if workerCount < 1 {
		workerCount = 4
	}

	cfg := SchedulerConfig{
		WorkerCount:  workerCount,
		QueueSize:    100,
		RateInterval: 200 * time.Millisecond,
	}

	globalScheduler = NewSchedulerV2(cfg, globalExecutor)
	globalScheduler.Start()

	var tasks []model.Task
	database.DB.Where("status = ?", model.TaskStatusEnabled).Find(&tasks)

	for _, task := range tasks {
		if err := globalScheduler.AddJob(&task); err != nil {
			log.Printf("failed to add task %d: %v", task.ID, err)
		}
	}

	startupCount := globalScheduler.EnqueueStartupTasks()
	log.Printf("scheduler v2 initialized with %d tasks", len(tasks))
	if startupCount > 0 {
		log.Printf("scheduler v2 enqueued %d startup task(s)", startupCount)
	}
}

func ShutdownSchedulerV2() {
	if globalScheduler != nil {
		globalScheduler.Stop()
	}
}

func GetSchedulerV2() *SchedulerV2 {
	return globalScheduler
}

func GetTaskExecutor() *TaskExecutor {
	return globalExecutor
}
