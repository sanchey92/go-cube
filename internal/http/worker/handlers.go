package worker

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sanchey92/go-cube/internal/http/utils"
	"github.com/sanchey92/go-cube/internal/task"
)

func (srv *Server) startHandler(w http.ResponseWriter, r *http.Request) {
	var te task.Event
	if err := utils.DecodeJSON(w, r, &te); err != nil {
		log.Printf("Decoding JSON error: %v", err)
		return
	}

	srv.worker.AddTask(&te.Task)
	log.Printf("Added task: %v\n", te.Task.ID)

	utils.WriteResponse(w, http.StatusCreated, &te.Task)
}

func (srv *Server) stopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		log.Printf("No taskID passed in request.\n")
		utils.WriteResponse(w, http.StatusBadRequest, utils.BadRequest(utils.ErrInvalidTaskID))
		return
	}

	tID, err := uuid.Parse(taskID)
	if err != nil {
		log.Printf("Invalid uuid: %q: %v\n", taskID, err)
		utils.WriteResponse(w, http.StatusBadRequest, utils.BadRequest(err))
		return
	}

	taskToStop, ok := srv.worker.GetTaskByID(tID)
	if !ok || taskToStop == nil {
		log.Printf("task %v not found\n", tID)
		utils.WriteResponse(w, http.StatusNotFound, utils.NotFound(utils.ErrTaskNotFound))
		return
	}

	if !task.ValidStateTransition(taskToStop.State, task.Completed) {
		log.Printf("Cannot stop task %v in state %v\n", tID, taskToStop.State)
		utils.WriteResponse(w, http.StatusConflict, utils.Conflict(utils.ErrInvalidTaskState))
		return
	}

	taskCopy := *taskToStop
	taskCopy.State = task.Completed

	srv.worker.AddTask(&taskCopy)
	utils.WriteResponse(w, http.StatusAccepted, nil)

}

func (srv *Server) getTasksHandler(w http.ResponseWriter, r *http.Request) {
	t := srv.worker.GetTasks()
	utils.WriteResponse(w, http.StatusOK, &t)
}

func (srv *Server) getStatsHandler(w http.ResponseWriter, r *http.Request) {
	s := srv.worker.GetStats()
	utils.WriteResponse(w, http.StatusOK, &s)
}
