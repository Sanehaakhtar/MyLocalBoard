package state

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// Path represents a drawing path
type Path struct {
	ID     string          `json:"id"`
	Points []fyne.Position `json:"points"`
	Color  string          `json:"color"`
	Stroke float32         `json:"stroke"`
}

// Clock represents a logical clock for CRDT operations
type Clock struct {
	counter int64
	mu      sync.Mutex
}

// Tick increments the clock and returns the new value
func (c *Clock) Tick() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counter++
	return c.counter
}

// Update updates the clock based on a received timestamp
func (c *Clock) Update(timestamp int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if timestamp > c.counter {
		c.counter = timestamp
	}
}

// PathOperation represents a CRDT operation for a drawing path
type PathOperation struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Timestamp int64     `json:"timestamp"`
	Path      Path      `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

// WhiteboardState is our CRDT data structure.
type WhiteboardState struct {
	siteID     string                    // A unique ID for this user's session
	clock      Clock                     // This user's logical clock
	paths      map[string]Path           // The actual set of paths, indexed by their unique ID
	operations map[string]PathOperation  // All operations we've seen
	mu         sync.RWMutex
}

// NewWhiteboardState creates and initializes a new CRDT state.
func NewWhiteboardState() *WhiteboardState {
	// Create a random site ID to prevent collisions between users.
	rand.Seed(time.Now().UnixNano())
	siteID := fmt.Sprintf("%d", rand.Intn(1000000))

	return &WhiteboardState{
		siteID:     siteID,
		paths:      make(map[string]Path),
		operations: make(map[string]PathOperation),
	}
}

// AddLocalPath takes a path drawn by the local user, assigns it a unique ID,
// adds it to the state, and returns the full path to be broadcast.
func (ws *WhiteboardState) AddLocalPath(p Path) Path {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Generate a unique ID using our site ID and logical clock.
	timestamp := ws.clock.Tick()
	p.ID = fmt.Sprintf("path-%s-%d", ws.siteID, timestamp)
	
	// Create operation
	op := PathOperation{
		ID:        p.ID,
		SiteID:    ws.siteID,
		Timestamp: timestamp,
		Path:      p,
		CreatedAt: time.Now(),
	}
	
	ws.paths[p.ID] = p
	ws.operations[p.ID] = op
	
	log.Printf("[CRDT] Local path added: %s", p.ID)
	return p
}

// AddRemotePath takes a path received from the network, merges it into our state,
// and returns true if this was a new path that should be drawn.
func (ws *WhiteboardState) AddRemotePath(p Path) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Check if we already have this path.
	if _, exists := ws.paths[p.ID]; exists {
		log.Printf("[CRDT] Path %s already exists, ignoring", p.ID)
		return false // It's a duplicate, do nothing.
	}

	// Extract timestamp from ID for clock synchronization
	// Format: "path-siteID-timestamp"
	var remoteSiteID string
	var remoteTimestamp int64
	fmt.Sscanf(p.ID, "path-%s-%d", &remoteSiteID, &remoteTimestamp)
	
	// Update our logical clock
	ws.clock.Update(remoteTimestamp)
	
	// Create operation record
	op := PathOperation{
		ID:        p.ID,
		SiteID:    remoteSiteID,
		Timestamp: remoteTimestamp,
		Path:      p,
		CreatedAt: time.Now(),
	}

	// Add to our state
	ws.paths[p.ID] = p
	ws.operations[p.ID] = op
	
	log.Printf("[CRDT] Remote path added: %s from site %s", p.ID, remoteSiteID)
	return true // Signal that the UI should be updated.
}

// GetAllPaths returns all paths in the current state
func (ws *WhiteboardState) GetAllPaths() []Path {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	paths := make([]Path, 0, len(ws.paths))
	for _, path := range ws.paths {
		paths = append(paths, path)
	}
	return paths
}

// GetSiteID returns this whiteboard's site ID
func (ws *WhiteboardState) GetSiteID() string {
	return ws.siteID
}

// GetOperationCount returns the number of operations processed
func (ws *WhiteboardState) GetOperationCount() int {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return len(ws.operations)
}

// RemovePath removes a path (for future delete operations)
func (ws *WhiteboardState) RemovePath(pathID string) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	if _, exists := ws.paths[pathID]; exists {
		delete(ws.paths, pathID)
		log.Printf("[CRDT] Path removed: %s", pathID)
		return true
	}
	return false
}

// Merge merges another whiteboard state into this one (for conflict resolution)
func (ws *WhiteboardState) Merge(other *WhiteboardState) []Path {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	newPaths := make([]Path, 0)
	
	// Merge all operations from the other state
	for opID, op := range other.operations {
		if _, exists := ws.operations[opID]; !exists {
			// This is a new operation
			ws.operations[opID] = op
			ws.paths[op.Path.ID] = op.Path
			ws.clock.Update(op.Timestamp)
			newPaths = append(newPaths, op.Path)
			log.Printf("[CRDT] Merged operation: %s", opID)
		}
	}
	
	return newPaths
}