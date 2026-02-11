package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the current state of an async task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// AsyncThresholdDays is the number of days beyond which a task is run asynchronously.
// When the time range of a backtest or download exceeds this value, the task is
// executed in the background and a task ID is returned immediately.
const AsyncThresholdDays = 30

// Task represents an asynchronous task.
type Task struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"` // "backtest", "download"
	Status    TaskStatus        `json:"status"`
	Progress  string            `json:"progress"`
	Percent   int               `json:"percent"` // 0-100
	Result    string            `json:"result,omitempty"`
	Error     string            `json:"error,omitempty"`
	Params    map[string]string `json:"params"`
	CreatedAt time.Time         `json:"createdAt"`
	StartedAt *time.Time        `json:"startedAt,omitempty"`
	EndedAt   *time.Time        `json:"endedAt,omitempty"`
}

// TaskManager manages async tasks.
type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewTaskManager creates a new task manager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*Task),
	}
}

// CreateTask creates a new task and returns its ID.
func (tm *TaskManager) CreateTask(taskType string, params map[string]string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := uuid.New().String()[:8]
	task := &Task{
		ID:        id,
		Type:      taskType,
		Status:    TaskStatusPending,
		Progress:  "waiting to start",
		Percent:   0,
		Params:    params,
		CreatedAt: time.Now(),
	}
	tm.tasks[id] = task
	return id
}

// StartTask marks a task as running.
func (tm *TaskManager) StartTask(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, ok := tm.tasks[id]; ok {
		t.Status = TaskStatusRunning
		now := time.Now()
		t.StartedAt = &now
		t.Progress = "running"
	}
}

// UpdateProgress updates the task progress info.
func (tm *TaskManager) UpdateProgress(id string, progress string, percent int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, ok := tm.tasks[id]; ok {
		t.Progress = progress
		t.Percent = percent
	}
}

// CompleteTask marks a task as completed with a result.
func (tm *TaskManager) CompleteTask(id string, result string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, ok := tm.tasks[id]; ok {
		t.Status = TaskStatusCompleted
		t.Result = result
		t.Progress = "completed"
		t.Percent = 100
		now := time.Now()
		t.EndedAt = &now
	}
}

// FailTask marks a task as failed with an error message.
func (tm *TaskManager) FailTask(id string, errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, ok := tm.tasks[id]; ok {
		t.Status = TaskStatusFailed
		t.Error = errMsg
		t.Progress = "failed"
		now := time.Now()
		t.EndedAt = &now
	}
}

// GetTask returns a task by ID.
func (tm *TaskManager) GetTask(id string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, ok := tm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task '%s' not found", id)
	}
	return t, nil
}

// ListTasks returns all tasks, optionally filtered by type and status.
func (tm *TaskManager) ListTasks(taskType string, status string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*Task
	for _, t := range tm.tasks {
		if taskType != "" && t.Type != taskType {
			continue
		}
		if status != "" && string(t.Status) != status {
			continue
		}
		result = append(result, t)
	}
	return result
}

// ShouldRunAsync determines if a task should run asynchronously
// based on the time range duration.
func ShouldRunAsync(start, end time.Time) bool {
	days := end.Sub(start).Hours() / 24
	return days > float64(AsyncThresholdDays)
}

// TaskResultJSON returns the task info as a JSON string suitable for MCP response.
func TaskResultJSON(task *Task) string {
	data, _ := json.MarshalIndent(task, "", "  ")
	return string(data)
}

// EstimatedSecondsPerDay is the rough estimation of how long (in seconds)
// it takes to process one day of data. Tuned per task type.
var estimatedSecondsPerDay = map[string]float64{
	"backtest":         0.5, // backtest is compute-heavy but data is local
	"backtest_managed": 0.5,
	"download":         2.0, // download is network-bound, slower per day
}

// ProgressEstimator runs a background ticker that updates the task's progress
// based on elapsed wall-clock time vs an estimated total duration derived from
// the data time range. Call the returned stop function (or close doneCh) when
// the actual operation finishes.
//
// dataStart/dataEnd define the data time range used to estimate total duration.
// The estimator caps at 95% — the final jump to 100% is done by CompleteTask.
func (tm *TaskManager) ProgressEstimator(taskID string, taskType string, dataStart, dataEnd time.Time) (doneCh chan struct{}) {
	doneCh = make(chan struct{})

	days := dataEnd.Sub(dataStart).Hours() / 24
	secsPerDay, ok := estimatedSecondsPerDay[taskType]
	if !ok {
		secsPerDay = 1.0
	}
	estimatedTotal := time.Duration(days*secsPerDay*1000) * time.Millisecond
	if estimatedTotal < 5*time.Second {
		estimatedTotal = 5 * time.Second
	}

	// Tick interval: ~2% of estimated total, clamped to [1s, 10s]
	tickInterval := time.Duration(float64(estimatedTotal) * 0.02)
	if tickInterval < 1*time.Second {
		tickInterval = 1 * time.Second
	}
	if tickInterval > 10*time.Second {
		tickInterval = 10 * time.Second
	}

	go func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		started := time.Now()

		for {
			select {
			case <-doneCh:
				return
			case <-ticker.C:
				elapsed := time.Since(started)
				// Use a logarithmic curve so progress slows down as it approaches 95%
				ratio := elapsed.Seconds() / estimatedTotal.Seconds()
				// Map ratio through 1 - e^(-2*ratio) so it approaches 1 asymptotically
				pct := (1 - math.Exp(-2*ratio)) * 95
				if pct < 5 {
					pct = 5
				}
				if pct > 95 {
					pct = 95
				}

				percent := int(pct)
				progress := fmt.Sprintf("processing... %.0f days range, elapsed %s",
					days, elapsed.Truncate(time.Second))

				tm.UpdateProgress(taskID, progress, percent)
			}
		}
	}()

	return doneCh
}
