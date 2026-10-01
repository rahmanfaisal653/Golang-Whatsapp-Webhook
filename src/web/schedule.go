package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ScheduledTask struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`          // "text", "image", "document", "video", "audio", "location", "contact", "status"
	To           string      `json:"to"`
	TargetName   string      `json:"target_name"`   // formatted or group name
	Payload      SendPayload `json:"payload"`
	ScheduledAt  string      `json:"scheduled_at"`  // human string e.g. "2026-10-01 20:30"
	DueTimestamp int64       `json:"due_timestamp"` // unix epoch seconds
	Status       string      `json:"status"`        // "PENDING", "SENT", "FAILED", "CANCELLED"
	CreatedAt    string      `json:"created_at"`
	SentAt       string      `json:"sent_at,omitempty"`
	Error        string      `json:"error,omitempty"`
}

type ScheduleManager struct {
	filePath string
	mu       sync.RWMutex
	tasks    []ScheduledTask
	waMgr    *WhatsAppManager
}

func NewScheduleManager(filePath string, waMgr *WhatsAppManager) *ScheduleManager {
	sm := &ScheduleManager{
		filePath: filePath,
		waMgr:    waMgr,
		tasks:    make([]ScheduledTask, 0),
	}
	_ = sm.load()
	return sm
}

func (s *ScheduleManager) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.tasks = make([]ScheduledTask, 0)
			return nil
		}
		return err
	}

	var tasks []ScheduledTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}
	s.tasks = tasks
	return nil
}

func (s *ScheduleManager) save() error {
	s.mu.RLock()
	tasks := s.tasks
	s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *ScheduleManager) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.processDueTasks(ctx)
			}
		}
	}()
}

func (s *ScheduleManager) Add(p SendPayload, dueTime time.Time) (*ScheduledTask, error) {
	bytes := make([]byte, 4)
	_, _ = rand.Read(bytes)
	taskID := fmt.Sprintf("sched_%s", hex.EncodeToString(bytes))

	msgType := strings.ToLower(strings.TrimSpace(p.Type))
	if msgType == "" {
		msgType = "text"
	}

	target := strings.TrimSpace(p.To)
	if msgType == "status" {
		target = "status@broadcast"
	}

	task := ScheduledTask{
		ID:           taskID,
		Type:         msgType,
		To:           target,
		TargetName:   target,
		Payload:      p,
		ScheduledAt:  dueTime.Format("2006-01-02 15:04"),
		DueTimestamp: dueTime.Unix(),
		Status:       "PENDING",
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}

	s.mu.Lock()
	s.tasks = append(s.tasks, task)
	s.mu.Unlock()

	_ = s.save()
	return &task, nil
}

func (s *ScheduleManager) List() []ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]ScheduledTask, len(s.tasks))
	copy(res, s.tasks)

	// Sort newest or due soonest first
	sort.Slice(res, func(i, j int) bool {
		return res[i].DueTimestamp > res[j].DueTimestamp
	})
	return res
}

func (s *ScheduleManager) Cancel(id string) error {
	s.mu.Lock()
	found := false
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			if s.tasks[i].Status == "PENDING" {
				s.tasks[i].Status = "CANCELLED"
				found = true
				break
			}
			s.mu.Unlock()
			return fmt.Errorf("task cannot be cancelled (status: %s)", s.tasks[i].Status)
		}
	}
	s.mu.Unlock()

	if !found {
		return errors.New("task not found")
	}
	return s.save()
}

func (s *ScheduleManager) Delete(id string) error {
	s.mu.Lock()
	idx := -1
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.tasks = append(s.tasks[:idx], s.tasks[idx+1:]...)
	}
	s.mu.Unlock()

	if idx < 0 {
		return errors.New("task not found")
	}
	return s.save()
}

func (s *ScheduleManager) ExecuteNow(ctx context.Context, id string) error {
	s.mu.RLock()
	var targetTask *ScheduledTask
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			targetTask = &s.tasks[i]
			break
		}
	}
	s.mu.RUnlock()

	if targetTask == nil {
		return errors.New("task not found")
	}

	err := s.waMgr.SendComplexMessage(ctx, targetTask.Payload)
	s.mu.Lock()
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			if err != nil {
				s.tasks[i].Status = "FAILED"
				s.tasks[i].Error = err.Error()
			} else {
				s.tasks[i].Status = "SENT"
				s.tasks[i].SentAt = time.Now().Format("2006-01-02 15:04:05")
				s.tasks[i].Error = ""
			}
			break
		}
	}
	s.mu.Unlock()

	_ = s.save()
	return err
}

func (s *ScheduleManager) processDueTasks(ctx context.Context) {
	now := time.Now().Unix()

	s.mu.RLock()
	var dueIDs []string
	for _, t := range s.tasks {
		if t.Status == "PENDING" && now >= t.DueTimestamp {
			dueIDs = append(dueIDs, t.ID)
		}
	}
	s.mu.RUnlock()

	if len(dueIDs) == 0 {
		return
	}

	for _, id := range dueIDs {
		s.mu.RLock()
		var p *SendPayload
		for i := range s.tasks {
			if s.tasks[i].ID == id && s.tasks[i].Status == "PENDING" {
				p = &s.tasks[i].Payload
				break
			}
		}
		s.mu.RUnlock()

		if p == nil {
			continue
		}

		log.Printf("[Scheduler] Dispatching scheduled message %s to %s (type: %s)", id, p.To, p.Type)
		err := s.waMgr.SendComplexMessage(ctx, *p)

		s.mu.Lock()
		for i := range s.tasks {
			if s.tasks[i].ID == id {
				if err != nil {
					s.tasks[i].Status = "FAILED"
					s.tasks[i].Error = err.Error()
				} else {
					s.tasks[i].Status = "SENT"
					s.tasks[i].SentAt = time.Now().Format("2006-01-02 15:04:05")
					s.tasks[i].Error = ""
				}
				break
			}
		}
		s.mu.Unlock()

		_ = s.save()
	}
}
