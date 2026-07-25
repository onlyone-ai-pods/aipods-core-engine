package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type FailedEvent struct {
	EventID       string    `json:"event_id"`
	EventType     EventType `json:"event_type"`
	Payload       interface{} `json:"payload"`
	FailureReason string    `json:"failure_reason"`
	RetryCount    int       `json:"retry_count"`
	FailedAt      time.Time `json:"failed_at"`
}

type DeadLetterQueue struct {
	mu          sync.Mutex
	failedQueue []FailedEvent
	maxRetries  int
}

func NewDeadLetterQueue(maxRetries int) *DeadLetterQueue {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &DeadLetterQueue{
		failedQueue: make([]FailedEvent, 0),
		maxRetries:  maxRetries,
	}
}

func (d *DeadLetterQueue) EnqueueFailedEvent(ctx context.Context, eventID string, evtType EventType, payload interface{}, err error, retryCount int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.failedQueue = append(d.failedQueue, FailedEvent{
		EventID:       eventID,
		EventType:     evtType,
		Payload:       payload,
		FailureReason: err.Error(),
		RetryCount:    retryCount,
		FailedAt:      time.Now(),
	})
	fmt.Printf("⚠️ Event [%s] of type [%s] sent to DLQ after %d retries: %v\n", eventID, evtType, retryCount, err)
}

func (d *DeadLetterQueue) GetDLQItems() []FailedEvent {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.failedQueue
}
