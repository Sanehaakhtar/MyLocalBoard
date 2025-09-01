package state

import (
	"encoding/json"
	"image/color"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type BoardState struct {
	mu      sync.RWMutex
	strokes map[string]Stroke
	order   []string // causal order approximation
}

func NewBoardState() *BoardState {
	return &BoardState{strokes: map[string]Stroke{}, order: []string{}}
}

func NewStrokeOp(points []Point, ts time.Time) Op {
	st := &Stroke{
		ID:     uuid.NewString(),
		Points: points,
		Color: color.Black,
		Width:  2,
		Time:   ts,
	}
	return Op{Type: OpInsertStroke, Stroke: st}
}

func (s *BoardState) Apply(op Op) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch op.Type {
	case OpInsertStroke:
		if op.Stroke == nil || op.Stroke.ID == "" { return false }
		if _, exists := s.strokes[op.Stroke.ID]; exists { return false }
		s.strokes[op.Stroke.ID] = *op.Stroke
		s.order = append(s.order, op.Stroke.ID)
	case OpDeleteStroke:
		if op.Target == "" { return false }
		if _, exists := s.strokes[op.Target]; !exists { return false }
		delete(s.strokes, op.Target)
		// remove from order
		tmp := s.order[:0]
		for _, id := range s.order {
			if id != op.Target { tmp = append(tmp, id) }
		}
		s.order = tmp
	default:
		return false
	}
	// keep a stable order by Lamport/Site if needed (not stored on state, simple append works for now)
	return true
}

func (s *BoardState) Strokes() []Stroke {
	s.mu.RLock(); defer s.mu.RUnlock()
	out := make([]Stroke, 0, len(s.order))
	for _, id := range s.order {
		if st, ok := s.strokes[id]; ok {
			out = append(out, st)
		}
	}
	// ensure deterministic order
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}

// Persistence (used by Transport)
func Save(path string) error {
	all := snapshot()
	b, _ := json.MarshalIndent(all, "", "  ")
	return os.WriteFile(path, b, 0644)
}

func Load(path string) error {
	b, err := os.ReadFile(path)
	if err != nil { return err }
	var loaded []Stroke
	if err := json.Unmarshal(b, &loaded); err != nil { return err }
	restore(loaded)
	return nil
}

// global snapshot/restore (simplifies wiring)
var globalState *BoardState
func init(){ globalState = NewBoardState() }
func snapshot() []Stroke { return globalState.Strokes() }
func restore(strokes []Stroke) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.strokes = map[string]Stroke{}
	globalState.order = []string{}
	for _, s := range strokes {
		globalState.strokes[s.ID] = s
		globalState.order = append(globalState.order, s.ID)
	}
}
