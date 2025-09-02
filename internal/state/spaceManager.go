package state

import (
	"sync"
	"time"
)

// DrawingArea represents a rectangular area on the canvas
type DrawingArea struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Position represents a point on the canvas
type Position struct {
	X float32
	Y float32
}

// SpaceRegion represents an occupied area on the canvas
type SpaceRegion struct {
	Area      DrawingArea
	Owner     string
	ClaimedAt time.Time
	PathIDs   []string // IDs of paths in this region
}

// SpaceManager handles drawing area allocation and conflict resolution
type SpaceManager struct {
	regions       []SpaceRegion
	allocatedAreas map[string]DrawingArea // clientID -> assigned area
	mu            sync.RWMutex
}

func NewSpaceManager() *SpaceManager {
	return &SpaceManager{
		regions:        make([]SpaceRegion, 0),
		allocatedAreas: make(map[string]DrawingArea),
	}
}

// AllocateSpace tries to allocate a drawing area for a client
func (sm *SpaceManager) AllocateSpace(clientID string, requestedArea DrawingArea) *DrawingArea {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if requested area overlaps with any occupied regions
	if sm.isAreaOccupied(requestedArea) {
		// Try to find alternative nearby area
		alternative := sm.findAlternativeArea(requestedArea)
		if alternative != nil {
			sm.allocatedAreas[clientID] = *alternative
			return alternative
		}
		return nil // No available space
	}

	// Area is free, allocate it
	sm.allocatedAreas[clientID] = requestedArea
	return &requestedArea
}

// ClaimSpace marks an area as occupied when someone draws there
func (sm *SpaceManager) ClaimSpace(pathID, owner string, points []Position) {
	if len(points) == 0 {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Calculate bounding box of the path
	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X, points[0].Y

	for _, point := range points {
		if point.X < minX {
			minX = point.X
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}

	// Add some padding around the path
	padding := float32(10)
	region := SpaceRegion{
		Area: DrawingArea{
			X:      minX - padding,
			Y:      minY - padding,
			Width:  maxX - minX + 2*padding,
			Height: maxY - minY + 2*padding,
		},
		Owner:     owner,
		ClaimedAt: time.Now(),
		PathIDs:   []string{pathID},
	}

	// Merge with existing region if owned by same user
	merged := false
	for i, existingRegion := range sm.regions {
		if existingRegion.Owner == owner && sm.regionsOverlap(region.Area, existingRegion.Area) {
			// Merge regions
			sm.regions[i] = sm.mergeRegions(existingRegion, region)
			merged = true
			break
		}
	}

	if !merged {
		sm.regions = append(sm.regions, region)
	}
}

// CanDrawInArea checks if a client has permission to draw in the area covered by the points
func (sm *SpaceManager) CanDrawInArea(clientID string, points []Position) bool {
	if len(points) == 0 {
		return false
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Host can draw anywhere
	if clientID == "host" {
		return true
	}

	// Check if client has an allocated area
	allocatedArea, hasAllocation := sm.allocatedAreas[clientID]
	if !hasAllocation {
		return false
	}

	// Check if all points are within the allocated area
	for _, point := range points {
		if !sm.pointInArea(point, allocatedArea) {
			return false
		}
	}

	// Check if area conflicts with existing drawings from other users
	for _, point := range points {
		if sm.pointInOccupiedRegion(point, clientID) {
			return false
		}
	}

	return true
}

// Helper methods

func (sm *SpaceManager) isAreaOccupied(area DrawingArea) bool {
	for _, region := range sm.regions {
		if sm.regionsOverlap(area, region.Area) {
			return true
		}
	}
	return false
}

func (sm *SpaceManager) regionsOverlap(a, b DrawingArea) bool {
	return !(a.X+a.Width < b.X || b.X+b.Width < a.X || 
		     a.Y+a.Height < b.Y || b.Y+b.Height < a.Y)
}

func (sm *SpaceManager) pointInArea(point Position, area DrawingArea) bool {
	return point.X >= area.X && point.X <= area.X+area.Width &&
		   point.Y >= area.Y && point.Y <= area.Y+area.Height
}

func (sm *SpaceManager) pointInOccupiedRegion(point Position, excludeOwner string) bool {
	for _, region := range sm.regions {
		if region.Owner != excludeOwner && sm.pointInArea(point, region.Area) {
			return true
		}
	}
	return false
}

func (sm *SpaceManager) findAlternativeArea(requestedArea DrawingArea) *DrawingArea {
	// Try areas around the requested area
	offsets := []struct{ dx, dy float32 }{
		{0, requestedArea.Height + 10},    // Below
		{0, -(requestedArea.Height + 10)}, // Above
		{requestedArea.Width + 10, 0},     // Right
		{-(requestedArea.Width + 10), 0},  // Left
	}

	for _, offset := range offsets {
		alternative := DrawingArea{
			X:      requestedArea.X + offset.dx,
			Y:      requestedArea.Y + offset.dy,
			Width:  requestedArea.Width,
			Height: requestedArea.Height,
		}

		if !sm.isAreaOccupied(alternative) && sm.isAreaInBounds(alternative) {
			return &alternative
		}
	}

	return nil
}

func (sm *SpaceManager) isAreaInBounds(area DrawingArea) bool {
	// Check if area is within reasonable canvas bounds
	return area.X >= 0 && area.Y >= 0 && 
		   area.X+area.Width <= 1200 && area.Y+area.Height <= 900
}

func (sm *SpaceManager) mergeRegions(existing, new SpaceRegion) SpaceRegion {
	minX := existing.Area.X
	if new.Area.X < minX {
		minX = new.Area.X
	}
	
	minY := existing.Area.Y
	if new.Area.Y < minY {
		minY = new.Area.Y
	}
	
	maxX := existing.Area.X + existing.Area.Width
	if new.Area.X+new.Area.Width > maxX {
		maxX = new.Area.X + new.Area.Width
	}
	
	maxY := existing.Area.Y + existing.Area.Height
	if new.Area.Y+new.Area.Height > maxY {
		maxY = new.Area.Y + new.Area.Height
	}

	merged := existing
	merged.Area = DrawingArea{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
	merged.PathIDs = append(merged.PathIDs, new.PathIDs...)
	
	return merged
}

// GetRegions returns all occupied regions (for debugging/visualization)
func (sm *SpaceManager) GetRegions() []SpaceRegion {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	regions := make([]SpaceRegion, len(sm.regions))
	copy(regions, sm.regions)
	return regions
}