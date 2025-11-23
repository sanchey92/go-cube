package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"

	client "github.com/sanchey92/go-cube/internal/client/http"
	"github.com/sanchey92/go-cube/internal/services/node"
	"github.com/sanchey92/go-cube/internal/services/task"
	"github.com/sanchey92/go-cube/pkg/errors"
)

const (
	maxRestartCount      = 3
	updateTasksInterval  = 15 * time.Second
	healthCheckInterval  = 60 * time.Second
	processTasksInterval = 10 * time.Second
	httpClientTimeout    = 10 * time.Second
)

type Scheduler interface {
	SelectCandidates(t *task.Task, nodes []*node.Node) []*node.Node
	Score(t *task.Task, nodes []*node.Node) map[string]float64
	Pick(scored map[string]float64, candidates []*node.Node) *node.Node
}

type Store interface {
	Put(key string, value interface{}) error
	Get(key string) (interface{}, error)
	List() (interface{}, error)
	Count() (int, error)
}

type Manager struct {
	pending       *queue.Queue
	workers       []string
	taskDB        Store
	eventDB       Store
	scheduler     Scheduler
	workerNodes   []*node.Node
	mu            sync.RWMutex
	workerTaskMap map[string][]uuid.UUID
	taskWorkerMap map[uuid.UUID]string
	httpClient    *client.HTTPClient
}

func New(workers []string, taskDB, eventDB Store, scheduler Scheduler) (*Manager, error) {
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	var nodes []*node.Node
	for _, worker := range workers {
		workerTaskMap[worker] = []uuid.UUID{}
		nAPI := fmt.Sprintf("http://%s", worker)
		n := node.NewNode(worker, nAPI, "worker")
		nodes = append(nodes, n)
	}

	return &Manager{
		pending:       queue.New(),
		workers:       workers,
		workerTaskMap: workerTaskMap,
		taskWorkerMap: taskWorkerMap,
		workerNodes:   nodes,
		scheduler:     scheduler,
		taskDB:        taskDB,
		eventDB:       eventDB,
		httpClient:    client.NewHTTPClient(httpClientTimeout),
	}, nil
}

func (m *Manager) AddTask(te task.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("[manager] adding task event %s to pending queue", te.ID)
	m.pending.Enqueue(te)
}

func (m *Manager) GetTasks() ([]*task.Task, error) {
	taskList, err := m.taskDB.List()
	if err != nil {
		return nil, fmt.Errorf("error getting list of tasks: %w", err)
	}

	tasks, ok := taskList.([]*task.Task)
	if !ok {
		return nil, fmt.Errorf("invalid type: expected []*task.Task, got %T", taskList)
	}

	return tasks, nil
}

func (m *Manager) GetTaskByID(taskID string) (*task.Task, error) {
	m.mu.RLock()
	defer m.mu.RLock()
	t, err := m.taskDB.Get(taskID)
	if err != nil {
		log.Printf("error getting task from db: %v", err)
		return nil, errors.ErrTaskNotFound
	}
	return t.(*task.Task), nil
}

func (m *Manager) SelectWorker(t *task.Task) (*node.Node, error) {
	candidates := m.scheduler.SelectCandidates(t, m.workerNodes)
	if candidates == nil || len(candidates) == 0 {
		log.Printf("[manager] no candidates available for task %s", t.ID)
		return nil, errors.ErrCandidatesDoesntExists
	}

	scores := m.scheduler.Score(t, candidates)
	if scores == nil || len(scores) == 0 {
		log.Printf("[manager] no scores available for task %s", t.ID)
		return nil, errors.ErrNoScored
	}

	selectedNode := m.scheduler.Pick(scores, candidates)
	if selectedNode == nil {
		return nil, fmt.Errorf("scheduler failed to pick a node")
	}

	return selectedNode, nil
}

func (m *Manager) UpdateTasks(ctx context.Context) {
	ticker := time.NewTicker(updateTasksInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[manager] UpdateTasks shutting down: %v", ctx.Err())
			return
		case <-ticker.C:
			log.Printf("[manager] checking for task updates from workers")
			m.updateTasks(ctx)
			log.Printf("[manager] task updates completed")
		}
	}
}

func (m *Manager) DoHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[manager] DoHealthChecks shutting down: %v", ctx.Err())
			return
		case <-ticker.C:
			log.Printf("[manager] performing task health checks")
			m.doHealthChecks(ctx)
			log.Printf("[manager] task health checks completed")
		}
	}
}

func (m *Manager) ProcessTasks(ctx context.Context) {
	ticker := time.NewTicker(processTasksInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[manager] ProcessTasks shutting down: %v", ctx.Err())
			return
		case <-ticker.C:
			log.Printf("[manager] processing tasks in the queue")
			if err := m.SendWork(ctx); err != nil {
				log.Printf("[manager] error processing task: %v", err)
			}
		}
	}
}

func (m *Manager) SendWork(ctx context.Context) error {
	if m.pending.Len() == 0 {
		log.Println("[manager] no work in the queue")
		return nil
	}

	e := m.pending.Dequeue()
	te, ok := e.(task.Event)
	if !ok {
		return fmt.Errorf("failed to convert dequeued item to task.Event, got type %T", e)
	}

	if err := m.eventDB.Put(te.ID.String(), &te); err != nil {
		return fmt.Errorf("failed to store task event %s: %w", te.ID.String(), err)
	}

	log.Printf("[manager] pulled task event %s off pending queue", te.ID)

	if workerName, exists := m.taskWorkerMap[te.Task.ID]; exists {
		return m.handleExistingTask(ctx, workerName, te)
	}

	return m.assignTaskToWorker(ctx, te)
}

func (m *Manager) updateTasks(ctx context.Context) {
	for _, workerAddr := range m.workers {
		select {
		case <-ctx.Done():
			log.Printf("[manager] updateTasks context cancelled")
			return
		default:
			log.Printf("[manager] checking worker %s for task updates", workerAddr)
			if err := m.updateWorkerTasks(ctx, workerAddr); err != nil {
				log.Printf("[manager] error updating tasks from worker %s: %v", workerAddr, err)
			}
		}
	}
}

func (m *Manager) updateWorkerTasks(ctx context.Context, workerAddr string) error {
	url := fmt.Sprintf("http://%s/tasks", workerAddr)

	var tasks []*task.Task
	if err := m.httpClient.Get(ctx, url, &tasks); err != nil {
		return fmt.Errorf("failed to get tasks from worker %s: %w", workerAddr, err)
	}

	for _, t := range tasks {
		if err := m.updateSingleTask(t); err != nil {
			log.Printf("[manager] failed to update task %s: %v", t.ID, err)
		}
	}

	return nil
}

func (m *Manager) updateSingleTask(t *task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	result, err := m.taskDB.Get(t.ID.String())
	if err != nil {
		return fmt.Errorf("failed to get task %s: %w", t.ID, err)
	}

	taskPersisted, ok := result.(*task.Task)
	if !ok {
		return fmt.Errorf("invalid type: expected *task.Task, got %T", result)
	}

	taskPersisted.State = t.State
	taskPersisted.StartTime = t.StartTime
	taskPersisted.FinishTime = t.FinishTime
	taskPersisted.ContainerID = t.ContainerID
	taskPersisted.HostPorts = t.HostPorts

	if err := m.taskDB.Put(taskPersisted.ID.String(), taskPersisted); err != nil {
		return fmt.Errorf("failed to save task %s: %w", taskPersisted.ID, err)
	}

	return nil
}

func (m *Manager) doHealthChecks(ctx context.Context) {
	tasks, err := m.GetTasks()
	if err != nil {
		log.Printf("[manager] error getting tasks for health check: %v", err)
		return
	}

	for _, t := range tasks {
		select {
		case <-ctx.Done():
			log.Printf("[manager] health checks context cancelled")
			return
		default:
			if t.State == task.Running && t.RestartCount < maxRestartCount {
				if err := m.checkTaskHealth(ctx, t); err != nil {
					log.Printf("[manager] task %s health check failed: %v", t.ID, err)
					if err := m.restartTask(ctx, t); err != nil {
						log.Printf("[manager] failed to restart task %s: %v", t.ID, err)
					}
				}
			} else if t.State == task.Failed && t.RestartCount < maxRestartCount {
				if err := m.restartTask(ctx, t); err != nil {
					log.Printf("[manager] failed to restart failed task %s: %v", t.ID, err)
				}
			}
		}
	}
}

func (m *Manager) checkTaskHealth(ctx context.Context, t *task.Task) error {
	m.mu.RLock()
	w, exists := m.taskWorkerMap[t.ID]
	m.mu.RUnlock()

	if !exists || w == "" {
		return fmt.Errorf("task %s has no assigned worker", t.ID)
	}

	log.Printf("[manager] calling health check for task %s: %s", t.ID, t.HealthCheck)

	worker := strings.Split(w, ":")
	if len(worker) == 0 {
		return fmt.Errorf("invalid worker address format: %s", w)
	}

	hostPort := getHostPortSafe(t.HostPorts)
	if hostPort == nil {
		log.Printf("[manager] task %s host port not collected yet, skipping health check", t.ID)
		return nil
	}

	url := fmt.Sprintf("http://%s:%s%s", worker[0], *hostPort, t.HealthCheck)
	if err := m.httpClient.Get(ctx, url, nil); err != nil {
		log.Printf("[manager] health check failed for task %s: %v", t.ID, err)
		return errors.ErrConnectingHealthCheck
	}

	log.Printf("[manager] task %s health check response: OK", t.ID)
	return nil
}

func (m *Manager) restartTask(ctx context.Context, t *task.Task) error {
	m.mu.Lock()
	w, exists := m.taskWorkerMap[t.ID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task %s has no assigned worker", t.ID)
	}
	m.mu.Unlock()

	t.State = task.Scheduled
	t.RestartCount++

	if err := m.taskDB.Put(t.ID.String(), t); err != nil {
		return fmt.Errorf("failed to persist task %s: %w", t.ID.String(), err)
	}

	te := &task.Event{
		ID:        uuid.New(),
		State:     task.Running,
		Timestamp: time.Now(),
		Task:      *t,
	}

	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("[manager] unable to marshal task object %s: %v", t.ID, err)
		return fmt.Errorf("failed to marshal task event: %w", err)
	}

	url := fmt.Sprintf("http://%s/tasks", w)
	var newTask task.Task
	if err = m.httpClient.Post(ctx, url, bytes.NewBuffer(data), &newTask); err != nil {
		log.Printf("[manager] error connecting to worker %s: %v", w, err)

		// Re-queue task on failure
		m.mu.Lock()
		m.pending.Enqueue(te)
		m.mu.Unlock()

		return fmt.Errorf("failed to post task to worker: %w", err)
	}

	log.Printf("[manager] task %s restarted successfully, response: %#v", t.ID, newTask)
	return nil
}

func (m *Manager) stopTask(ctx context.Context, worker, taskID string) error {
	url := fmt.Sprintf("http://%s/tasks/%s", worker, taskID)
	if err := m.httpClient.Delete(ctx, url); err != nil {
		log.Printf("[manager] failed to delete task %s on worker %s: %v", taskID, worker, err)
		return fmt.Errorf("failed to delete task %s: %w", taskID, err)
	}

	log.Printf("[manager] task %s has been scheduled to be stopped on worker %s", taskID, worker)
	return nil
}

func (m *Manager) handleExistingTask(ctx context.Context, workerName string, te task.Event) error {
	result, err := m.taskDB.Get(te.Task.ID.String())
	if err != nil {
		return fmt.Errorf("unable to retrieve task %s: %w", te.Task.ID.String(), err)
	}

	persistedTask, ok := result.(*task.Task)
	if !ok {
		return fmt.Errorf("invalid type: expected *task.Task, got %T", result)
	}

	if te.State == task.Completed && task.ValidStateTransition(persistedTask.State, te.State) {
		if err := m.stopTask(ctx, workerName, te.Task.ID.String()); err != nil {
			return fmt.Errorf("failed to stop task %s: %w", te.Task.ID.String(), err)
		}
		return nil
	}

	log.Printf("[manager] invalid request: existing task %s is in state %v and cannot transition to state %v",
		persistedTask.ID.String(), persistedTask.State, te.State)
	return nil
}

func (m *Manager) assignTaskToWorker(ctx context.Context, te task.Event) error {
	t := te.Task

	worker, err := m.SelectWorker(&t)

	if err != nil {
		return fmt.Errorf("failed to select worker for task %s: %w", t.ID, err)
	}

	log.Printf("[manager] selected worker %s for task %s", worker.Name, t.ID)

	m.workerTaskMap[worker.Name] = append(m.workerTaskMap[worker.Name], t.ID)
	m.taskWorkerMap[t.ID] = worker.Name

	t.State = task.Scheduled
	if err = m.taskDB.Put(t.ID.String(), &t); err != nil {
		return fmt.Errorf("failed to persist task %s: %w", t.ID.String(), err)
	}

	url := fmt.Sprintf("http://%s/tasks", worker.Name)

	payload, err := createTaskPayloadSafe(te)
	if err != nil {
		return fmt.Errorf("failed to create task payload: %w", err)
	}

	var response task.Task
	if err := m.httpClient.Post(ctx, url, payload, &response); err != nil {
		log.Printf("[manager] error sending task to worker %s: %v", worker.Name, err)
		m.pending.Enqueue(te)
		return fmt.Errorf("failed to send task to worker: %w", err)
	}

	worker.TaskCount++
	log.Printf("[manager] received response from worker %s: %#v", worker.Name, response)

	return nil
}

func createTaskPayloadSafe(te task.Event) (io.Reader, error) {
	data, err := json.Marshal(te)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task event: %w", err)
	}
	return bytes.NewBuffer(data), nil
}

func getHostPortSafe(ports nat.PortMap) *string {
	for k := range ports {
		if len(ports[k]) > 0 {
			return &ports[k][0].HostPort
		}
	}
	return nil
}
