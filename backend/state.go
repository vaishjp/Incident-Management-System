package main

import "errors"

//  The State Interface
type IncidentState interface {
	Acknowledge(w *WorkItem) error
	Resolve(w *WorkItem) error
	Close(w *WorkItem) error
	Name() string
}

//  OPEN STATE
type OpenState struct{}
func (s *OpenState) Acknowledge(w *WorkItem) error {
	w.SetState(&InvestigatingState{})
	return nil
}
func (s *OpenState) Resolve(w *WorkItem) error { return errors.New("cannot resolve: incident must be acknowledged first") }
func (s *OpenState) Close(w *WorkItem) error   { return errors.New("cannot close: incident is still open") }
func (s *OpenState) Name() string              { return "OPEN" }

//  INVESTIGATING STATE
type InvestigatingState struct{}
func (s *InvestigatingState) Acknowledge(w *WorkItem) error { return errors.New("already investigating") }
func (s *InvestigatingState) Resolve(w *WorkItem) error {
	w.SetState(&ResolvedState{})
	return nil
}
func (s *InvestigatingState) Close(w *WorkItem) error { return errors.New("cannot close: RCA must be submitted to resolve first") }
func (s *InvestigatingState) Name() string            { return "INVESTIGATING" }

//  RESOLVED STATE (RCA Submitted)
type ResolvedState struct{}
func (s *ResolvedState) Acknowledge(w *WorkItem) error { return errors.New("incident is already resolved") }
func (s *ResolvedState) Resolve(w *WorkItem) error     { return errors.New("incident is already resolved") }
func (s *ResolvedState) Close(w *WorkItem) error {
	w.SetState(&ClosedState{})
	return nil
}
func (s *ResolvedState) Name() string { return "RESOLVED" }

//  CLOSED STATE (Terminal)
type ClosedState struct{}
func (s *ClosedState) Acknowledge(w *WorkItem) error { return errors.New("incident is closed") }
func (s *ClosedState) Resolve(w *WorkItem) error     { return errors.New("incident is closed") }
func (s *ClosedState) Close(w *WorkItem) error       { return errors.New("incident is already closed") }
func (s *ClosedState) Name() string                  { return "CLOSED" }
