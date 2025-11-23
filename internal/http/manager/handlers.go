package manager

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sanchey92/go-cube/internal/services/task"
	"github.com/sanchey92/go-cube/pkg/errors"
	"github.com/sanchey92/go-cube/pkg/utils"
)

func (srv *Server) startTaskHandler(w http.ResponseWriter, r *http.Request) {
	te := task.Event{}
	if err := utils.DecodeJSON(w, r, &te); err != nil {
		log.Printf("Decoding JSON error: %v", err)
		return
	}

	srv.manager.AddTask(te)
	log.Printf("Added task %v\n", te.Task.ID)
	utils.WriteResponse(w, http.StatusCreated, &te.Task)
}

func (srv *Server) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks, err := srv.manager.GetTasks()
	if err != nil {
		log.Printf("error getting tasks from manager: %v", err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, &tasks)
}

func (srv *Server) stopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		log.Printf("no taskID passed in request.\n")
		utils.WriteResponse(w, http.StatusBadRequest, errors.BadRequest(errors.ErrInvalidTaskID))
		return
	}

	tID, err := uuid.Parse(taskID)
	if err != nil {
		log.Printf("Invalid uuid: %q: %v\n", taskID, err)
		utils.WriteResponse(w, http.StatusBadRequest, errors.BadRequest(err))
		return
	}

	taskToStop, err := srv.manager.GetTaskByID(tID.String())
	if err != nil {
		log.Printf("task not found: %v", err)
		utils.WriteResponse(w, http.StatusBadRequest, errors.BadRequest(errors.ErrTaskNotFound))
		return
	}

	taskCopy := taskToStop

	te := task.Event{
		ID:        uuid.New(),
		State:     task.Completed,
		Timestamp: time.Now(),
	}

	te.Task = *taskCopy
	srv.manager.AddTask(te)

	log.Printf("added task event %s to stop task %v\n", te.ID, taskCopy.ID)
}
