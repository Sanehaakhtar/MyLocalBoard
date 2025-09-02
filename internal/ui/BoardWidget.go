package ui

import (
	"fmt"
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Path struct now includes an OwnerID
type Path struct {
	ID      string          `json:"id"`
	OwnerID string          `json:"owner_id"` // The ID of the user who drew this
	Points  []fyne.Position `json:"points"`
	Color   string          `json:"color"`
	Stroke  float32         `json:"stroke"`
}

type BoardWidget struct {
	widget.BaseWidget
	paths           []*Path
	mu              sync.RWMutex
	currentPath     *Path
	panX, panY      float32
	drawing         bool
	currentColor    string
	currentStroke   float32
	LocalClientID   string // The ID of the person using this specific window
	OnNewPath       func(p Path)
	OnClear         func()
	remotePathsChan chan Path
	clearChan       chan string // Now carries the OwnerID to clear
	statusChan      chan string
	statusBar       *widget.Label
}

var _ fyne.Widget = (*BoardWidget)(nil)
var _ fyne.Draggable = (*BoardWidget)(nil)
var _ desktop.Mouseable = (*BoardWidget)(nil)

func NewBoardWidget() *BoardWidget {
	b := &BoardWidget{
		paths:           make([]*Path, 0),
		currentColor:    "black",
		currentStroke:   3.0,
		remotePathsChan: make(chan Path, 100),
		clearChan:       make(chan string, 10),
		statusChan:      make(chan string, 10),
		statusBar:       widget.NewLabel("Ready"),
	}
	b.ExtendBaseWidget(b)
	go b.listenForUpdates()
	return b
}

func (b *BoardWidget) listenForUpdates() {
	for {
		select {
		case path := <-b.remotePathsChan:
			b.mu.Lock(); b.paths = append(b.paths, &path); b.mu.Unlock()
			b.Refresh()
		case ownerToClear := <-b.clearChan:
			b.clearPathsByOwner(ownerToClear) // Call the new selective clear
			log.Printf("[UI] Safely cleared paths for owner %s.", ownerToClear)
		case text := <-b.statusChan:
			b.statusBar.SetText(text)
		}
	}
}

// SetLocalClientID gives the board its own identity.
func (b *BoardWidget) SetLocalClientID(id string) {
	b.LocalClientID = id
}

// clearPathsByOwner is the new core logic for selective clearing.
func (b *BoardWidget) clearPathsByOwner(ownerID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Create a new slice, keeping only the paths that do NOT match the ownerID
	filteredPaths := make([]*Path, 0)
	for _, path := range b.paths {
		if path.OwnerID != ownerID {
			filteredPaths = append(filteredPaths, path)
		}
	}
	// Replace the old list with the filtered list
	b.paths = filteredPaths
	b.Refresh()
}


func (b *BoardWidget) AddRemotePath(p Path) { b.remotePathsChan <- p }
func (b *BoardWidget) ClearRemote(ownerID string) { b.clearChan <- ownerID }
func (b *BoardWidget) SetStatus(text string) { b.statusChan <- text }

// ClearPaths is called by the local UI button.
func (b *BoardWidget) ClearPaths() {
	// It clears its own paths directly and then fires the network event.
	b.clearPathsByOwner(b.LocalClientID)
	if b.OnClear != nil {
		b.OnClear()
	}
}
func (b *BoardWidget) SetColor(c color.Color) {
	b.mu.Lock(); defer b.mu.Unlock()
	switch c {
	case color.Black: b.currentColor = "black"
	case color.RGBA{R: 255, A: 255}: b.currentColor = "red"
	case color.RGBA{B: 255, A: 255}: b.currentColor = "blue"
	case color.RGBA{G: 255, A: 255}: b.currentColor = "green"
	default: b.currentColor = "black"
	}
}
func (b *BoardWidget) SetStroke(s float32) { b.mu.Lock(); defer b.mu.Unlock(); b.currentStroke = s }
func (b *BoardWidget) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.drawing = true; adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.mu.RLock(); colorToUse := b.currentColor; strokeToUse := b.currentStroke; b.mu.RUnlock()
		
		// Stamp the path with the owner's ID
		b.currentPath = &Path{ ID: fmt.Sprintf("path-%d", time.Now().UnixNano()), OwnerID: b.LocalClientID, Points: []fyne.Position{adjustedPos}, Color: colorToUse, Stroke: strokeToUse }
	}
}
func (b *BoardWidget) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary && b.drawing {
		b.drawing = false
		if b.currentPath != nil && len(b.currentPath.Points) > 1 {
			// Add the finalized path to the local list first
			b.mu.Lock(); b.paths = append(b.paths, b.currentPath); b.mu.Unlock()
			// Then send it over the network
			if b.OnNewPath != nil { b.OnNewPath(*b.currentPath) }
		}
		b.currentPath = nil; b.Refresh()
	}
}
func (b *BoardWidget) Dragged(e *fyne.DragEvent) {
	if b.drawing && b.currentPath != nil {
		adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.currentPath.Points = append(b.currentPath.Points, adjustedPos)
		b.Refresh()
	} else if !b.drawing {
		b.panX += e.Dragged.DX; b.panY += e.Dragged.DY; b.Refresh()
	}
}
// Renderer (Unchanged)
func (b *BoardWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &boardWidgetRenderer{board: b}; r.background = canvas.NewRectangle(color.White); return r
}
type boardWidgetRenderer struct { board *BoardWidget; background *canvas.Rectangle }
func (r *boardWidgetRenderer) Objects() []fyne.CanvasObject {
    r.board.mu.RLock(); defer r.board.mu.RUnlock()
    objects := []fyne.CanvasObject{r.background}
    pathsToRender := make([]*Path, len(r.board.paths)); copy(pathsToRender, r.board.paths)
    if r.board.drawing && r.board.currentPath != nil { pathsToRender = append(pathsToRender, r.board.currentPath) }
    for _, p := range pathsToRender {
        var pathColor color.Color = color.Black
        if p.Color == "red" { pathColor = color.RGBA{R: 255, A: 255}
        } else if p.Color == "blue" { pathColor = color.RGBA{B: 255, A: 255}
        } else if p.Color == "green" { pathColor = color.RGBA{G: 255, A: 255} }
        if len(p.Points) > 1 {
            for i := 0; i < len(p.Points)-1; i++ {
                segment := canvas.NewLine(pathColor); segment.StrokeWidth = p.Stroke
                segment.Position1 = fyne.NewPos(p.Points[i].X+r.board.panX, p.Points[i].Y+r.board.panY)
                segment.Position2 = fyne.NewPos(p.Points[i+1].X+r.board.panX, p.Points[i+1].Y+r.board.panY)
                objects = append(objects, segment)
            }
        }
    }
    return objects
}
func (r *boardWidgetRenderer) Refresh() { canvas.Refresh(r.board) }
func (b *BoardWidget) MouseIn(*desktop.MouseEvent) {}
func (b *BoardWidget) MouseOut() {}
func (b *BoardWidget) MouseMoved(*desktop.MouseEvent) {}
func (b *BoardWidget) DragEnd() {}
func (r *boardWidgetRenderer) Destroy() {}
func (r *boardWidgetRenderer) Layout(size fyne.Size) { r.background.Resize(size) }
func (r *boardWidgetRenderer) MinSize() fyne.Size { return fyne.NewSize(300, 300) }
func (b *BoardWidget) Scrolled(e *fyne.ScrollEvent) { b.panX += e.Scrolled.DX; b.panY += e.Scrolled.DY; b.Refresh() }