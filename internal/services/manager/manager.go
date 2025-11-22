package manager

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"

	client "github.com/sanchey92/go-cube/internal/client/http"
	"github.com/sanchey92/go-cube/internal/services/node"
	"github.com/sanchey92/go-cube/internal/services/task"
	"github.com/sanchey92/go-cube/pkg/errors"
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
	lastWorker    int
	httpClient    *client.HTTPClient
}

func New(workers []string, taskDB, eventDB Store, scheduler Scheduler) *Manager {
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	var nodes []*node.Node
	for w := range workers {
		workerTaskMap[workers[w]] = []uuid.UUID{}
		nAPI := fmt.Sprintf("http://%v", workers[w])
		n := node.NewNode(workers[w], nAPI, "worker")
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
		httpClient:    client.NewHTTPClient(10 * time.Second),
	}
}

func (m *Manager) AddTask(te *task.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("add task event %v to pending queue", te)
	m.pending.Enqueue(te)
}

func (m *Manager) GetTasks() []*task.Task {
	taskList, err := m.taskDB.List()
	if err != nil {
		log.Printf("error gettig list of tasks: %v", err)
		return nil
	}
	return taskList.([]*task.Task)
}

func (m *Manager) SelectWorker(t *task.Task) (*node.Node, error) {
	candidates := m.scheduler.SelectCandidates(t, m.workerNodes)
	if candidates == nil {
		log.Printf(errors.ErrCandidatesDoesntExists.Error())
		return nil, errors.ErrCandidatesDoesntExists
	}
	scores := m.scheduler.Score(t, candidates)
	if scores == nil {
		fmt.Printf(errors.ErrNoScored.Error())
		return nil, errors.ErrNoScored
	}

	selectedNode := m.scheduler.Pick(scores, candidates)

	return selectedNode, nil
}

func (m *Manager) UpdateTasks(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("context is done")
			return
		case <-ticker.C:
			log.Printf("checking for task updates from workers")
			m.updateTasks(ctx)
			log.Printf("task updates completed")
		}
	}
}

func (m *Manager) updateTasks(ctx context.Context) {
	for _, workerAddr := range m.workers {
		log.Printf("checking worker %s for task updates", workerAddr)

		if err := m.updateWorkerTasks(ctx, workerAddr); err != nil {
			log.Printf("error updating tasks from worker %s: %v", workerAddr, err)
		}
	}
}

func (m *Manager) updateWorkerTasks(ctx context.Context, workerAddr string) error {
	url := fmt.Sprintf("http://%s/tasks", workerAddr)

	var tasks []*task.Task
	if err := m.httpClient.Get(ctx, url, &tasks); err != nil {
		return errors.ErrFailedGetTasks
	}

	for _, t := range tasks {
		if err := m.updateSingleTask(t); err != nil {
			log.Printf("failed to update task %s: %v", t.ID, err)
		}
	}

	return nil
}

func (m *Manager) updateSingleTask(t *task.Task) error {
	result, err := m.taskDB.Get(t.ID.String())
	if err != nil {
		return errors.ErrFailedGetTasks
	}

	taskPersisted, ok := result.(*task.Task)
	if !ok {
		return errors.ErrInvalidTypeExpected
	}

	taskPersisted.State = t.State
	taskPersisted.StartTime = t.StartTime
	taskPersisted.FinishTime = t.FinishTime
	taskPersisted.ContainerID = t.ContainerID
	taskPersisted.HostPorts = t.HostPorts

	if err = m.taskDB.Put(taskPersisted.ID.String(), taskPersisted); err != nil {
		return errors.ErrFailedSave
	}

	return nil
}
