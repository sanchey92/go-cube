package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"

	"github.com/sanchey92/go-cube/internal/models"
	"github.com/sanchey92/go-cube/internal/task"
)

type Container interface {
	Run(ctx context.Context, t *task.Task) *models.ContainerResult
	StopAndRemove(ctx context.Context, id string) *models.ContainerResult
	Inspect(ctx context.Context, id string) *models.InspectResponse
}

type Worker struct {
	Name            string
	queue           *queue.Queue
	mu              sync.RWMutex
	db              map[uuid.UUID]*task.Task
	containerClient Container
	stats           *Stats
	taskCount       atomic.Int32
}

func New(name string, containerClient Container) *Worker {
	return &Worker{
		Name:            name,
		containerClient: containerClient,
		db:              make(map[uuid.UUID]*task.Task),
		queue:           queue.New(),
	}
}

func (w *Worker) GetStats() *Stats {
	return w.stats
}

func (w *Worker) AddTask(t *task.Task) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue.Enqueue(t)
}

func (w *Worker) GetTasks() []*task.Task {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var tasks []*task.Task
	for _, t := range w.db {
		tasks = append(tasks, t)
	}

	return tasks
}

func (w *Worker) GetTaskByID(taskID uuid.UUID) (*task.Task, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	t, ok := w.db[taskID]
	return t, ok
}

func (w *Worker) StartTask(ctx context.Context, t *task.Task) *models.ContainerResult {
	result := w.containerClient.Run(ctx, t)

	w.mu.Lock()
	defer w.mu.Unlock()

	if result.Err != nil {
		log.Printf("Error running task: %v: %v\n", t.ID, result.Err)
		t.State = task.Failed
		w.db[t.ID] = t
		return result
	}

	t.ContainerID = result.ContainerID
	t.State = task.Running
	w.db[t.ID] = t
	return result
}

func (w *Worker) StopTask(ctx context.Context, t *task.Task) *models.ContainerResult {
	result := w.containerClient.StopAndRemove(ctx, t.ContainerID)

	w.mu.Lock()
	defer w.mu.Unlock()

	t.FinishTime = time.Now().UTC()
	if result.Err != nil {
		log.Printf("Error stopping task %v: %v\n", t.ID, result.Err)
		t.State = task.Failed
	} else {
		t.State = task.Completed
	}

	w.db[t.ID] = t
	log.Printf("Stopped and removed container %v for task %v with state %v\n", t.ContainerID, t.ID, t.State)
	return result
}

func (w *Worker) CollectStats(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("context is done")
			return
		case <-ticker.C:
			log.Println("collecting stats")
			w.mu.Lock()
			w.stats = GetStats()
			w.mu.Unlock()
			w.taskCount.Add(1)
		}
	}
}

func (w *Worker) UpdateTasks(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("context is done")
			return
		case <-ticker.C:
			log.Println("Checking status of tasks")
			w.updateTasks(ctx)
			log.Println("Sleeping for 15 seconds")
		}
	}
}

func (w *Worker) RunTasks(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("context is done")
			return
		case <-ticker.C:
			w.mu.RLock()
			queueLen := w.queue.Len()
			w.mu.RUnlock()

			if queueLen == 0 {
				log.Printf("no tasks to process currently.\n")
				continue
			}
			if result := w.runTask(ctx); result != nil && result.Err != nil {
				log.Printf("Error running task: %v", result.Err)
			}
		}
	}
}

func (w *Worker) updateTasks(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, t := range w.db {
		if t.State == task.Running {
			r := w.containerClient.Inspect(ctx, t.ContainerID)
			if r.Err != nil {
				log.Printf("Error inspecting container for task %s: %v\n", id, r.Err)
				continue
			}
			if r.Container == nil {
				log.Printf("No container for running task: %s", id)
				w.db[id].State = task.Failed
				w.db[id].FinishTime = time.Now().UTC()
				continue
			}
			if r.Container.State.Status == "exited" {
				log.Printf("Container for task %s has exited with status: %s", id, r.Container.State.Status)
				w.db[id].FinishTime = time.Now().UTC()
				if r.Container.State.ExitCode == 0 {
					w.db[id].State = task.Completed
				} else {
					w.db[id].State = task.Failed
				}
				continue
			}
			w.db[id].HostPorts = r.Container.NetworkSettings.NetworkSettingsBase.Ports
		}
	}
}

func (w *Worker) runTask(ctx context.Context) *models.ContainerResult {
	w.mu.Lock()
	t := w.queue.Dequeue()
	if t == nil {
		w.mu.Unlock()
		log.Println("No tasks in the queue")
		return &models.ContainerResult{}
	}

	taskQueued, ok := t.(*task.Task)
	if !ok {
		w.mu.Unlock()
		log.Println("Invalid task type in queue")
		return &models.ContainerResult{Err: fmt.Errorf("invalid task type")}
	}

	taskPersisted := w.db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = taskQueued
		w.db[taskQueued.ID] = taskQueued
	}

	if !task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		w.mu.Unlock()
		err := fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		return &models.ContainerResult{Err: err}
	}

	var result *models.ContainerResult

	switch taskQueued.State {
	case task.Scheduled:
		if taskQueued.ContainerID != "" {
			result = w.stopTaskLocked(ctx, taskQueued)
			if result.Err != nil {
				log.Printf("Error stopping existing container: %v\n", result.Err)
			}
		}
		result = w.startTaskLocked(ctx, taskQueued)
	case task.Completed:
		result = w.stopTaskLocked(ctx, taskQueued)
	default:
		w.mu.Unlock()
		log.Printf("Invalid state for task execution. taskPersisted: %v, taskQueued: %v\n", taskPersisted.State, taskQueued.State)
		result = &models.ContainerResult{Err: fmt.Errorf("invalid state: %v", taskQueued.State)}
		return result
	}

	w.mu.Unlock()
	return result
}

func (w *Worker) startTaskLocked(ctx context.Context, t *task.Task) *models.ContainerResult {
	w.mu.Unlock()
	result := w.containerClient.Run(ctx, t)
	w.mu.Lock()

	if result.Err != nil {
		log.Printf("Error running task: %v: %v\n", t.ID, result.Err)
		t.State = task.Failed
		w.db[t.ID] = t
		return result
	}

	t.ContainerID = result.ContainerID
	t.State = task.Running
	w.db[t.ID] = t
	return result
}

func (w *Worker) stopTaskLocked(ctx context.Context, t *task.Task) *models.ContainerResult {
	w.mu.Unlock()
	result := w.containerClient.StopAndRemove(ctx, t.ContainerID)
	w.mu.Lock()

	t.FinishTime = time.Now().UTC()
	if result.Err != nil {
		log.Printf("Error stopping task %v: %v\n", t.ID, result.Err)
		t.State = task.Failed
	} else {
		t.State = task.Completed
	}

	w.db[t.ID] = t
	log.Printf("Stopped and removed container %v for task %v with state %v\n", t.ContainerID, t.ID, t.State)
	return result
}
