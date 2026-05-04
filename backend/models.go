package main

import (
	"time"

	"gorm.io/gorm"
)

// 🐘 PostgreSQL: Structured Transactional Data
type WorkItem struct {
	gorm.Model
	// 👉 FIX 21: Database Indexes for fast dashboard querying & timeline generation
	ComponentID     string    `json:"component_id" gorm:"index"`
	ErrorType       string    `json:"error_type"`
	Severity        string    `json:"severity"`
	Status          string    `json:"status" gorm:"index"` 
	RCAs            []RCA     `json:"rcas" gorm:"foreignKey:WorkItemID"`
	ResolvedAt      time.Time `json:"resolved_at"`
	
	// 👉 FIX 22: Precise MTTR tracking based on the actual hardware signal, not DB creation time
	FirstSignalTime time.Time `json:"first_signal_time" gorm:"index"`
	MTTR            int64     `json:"mttr_minutes"`

	// State Machine Object (Not saved to DB, used in memory)
	State IncidentState `json:"-" gorm:"-"`
}

type RCA struct {
	gorm.Model
	WorkItemID  uint   `json:"work_item_id" gorm:"index"`
	RootCause   string `json:"root_cause"`
	FixApplied  string `json:"fix_applied"`
	SubmittedBy string `json:"submitted_by"`
}

// 🍃 MongoDB: Unstructured Audit Log Data
type Signal struct {
	ID          string `json:"id" bson:"_id,omitempty"`
	ComponentID string `json:"component_id" bson:"component_id"`
	ErrorType   string `json:"error_type" bson:"error_type"`
	Severity    string `json:"severity" bson:"severity"`
	Message     string `json:"message" bson:"message"`
	Timestamp   string `json:"timestamp" bson:"timestamp"`

	// Relational link back to the Postgres WorkItem
	WorkItemID uint `json:"work_item_id" bson:"work_item_id"`
}

// State Pattern Loaders
func (w *WorkItem) LoadState() {
	switch w.Status {
	case "OPEN":
		w.State = &OpenState{}
	case "INVESTIGATING":
		w.State = &InvestigatingState{}
	case "RESOLVED":
		w.State = &ResolvedState{}
	case "CLOSED":
		w.State = &ClosedState{}
	default:
		w.State = &OpenState{}
	}
}

func (w *WorkItem) SetState(s IncidentState) {
	w.State = s
	w.Status = s.Name()
}
