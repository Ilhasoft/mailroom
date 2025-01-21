package queues

import (
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
)

// Task is a wrapper for encoding a task
type Task struct {
	Type       string          `json:"type"`
	OwnerID    int             `json:"-"`
	Task       json.RawMessage `json:"task"`
	QueuedOn   time.Time       `json:"queued_on"`
	ErrorCount int             `json:"error_count,omitempty"`
}

// Fair is a queue that supports fair distribution of tasks between owners
type Fair interface {
	Push(rc redis.Conn, taskType string, ownerID int, task any, priority bool) error
	Pop(rc redis.Conn) (*Task, error)
	Done(rc redis.Conn, ownerID int) error
	Pause(rc redis.Conn, ownerID int) error
	Resume(rc redis.Conn, ownerID int) error
	Owners(rc redis.Conn) ([]int, error)
	Size(rc redis.Conn) (int, error)
}
