package ui

import (
	"image/color"
	"log"
	"sync"
	"time"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	// "fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Path struct now uses a string for color for network safety.
type Path struct {
	ID     string          `json:"id"`
	Points []fyne.Position `json:"points"`
	Color  string          `json:"color"`
	Stroke float32         `json:"stroke"`
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
	OnNewPath       func(p Path)
	// Channels for safe, concurrent UI updates
	remotePathsChan chan Path
	statusChan      chan string
	statusBar       *widget.Label // Reference to the status bar
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
		statusChan:      make(chan string, 10),
		statusBar:       widget.NewLabel("Ready"), // Create the status bar here
	}
	b.ExtendBaseWidget(b)
	go b.listenForRemotePaths()
	return b
}

func (b *BoardWidget) listenForRemotePaths() {
	for path := range b.remotePathsChan {
		b.mu.Lock()
		b.paths = append(b.paths, &path)
		b.mu.Unlock()
		b.Refresh()
		log.Println("[UI] Safely added remote path and refreshed.")
	}
}

// AddRemotePath is a thread-safe way to add a path from the network.
func (b *BoardWidget) AddRemotePath(p Path) {
	b.remotePathsChan <- p
}

// SetStatus is a thread-safe way to update the status bar text.
func (b *BoardWidget) SetStatus(text string) {
	b.statusChan <- text
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
func (b *BoardWidget) SetStroke(s float32) {
	b.mu.Lock(); defer b.mu.Unlock()
	b.currentStroke = s
}
func (b *BoardWidget) ClearPaths() {
	b.mu.Lock(); defer b.mu.Unlock()
	b.paths = make([]*Path, 0)
	b.Refresh()
	// TODO: Broadcast a "clear" message
}

func (b *BoardWidget) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.drawing = true; adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.mu.RLock(); colorToUse := b.currentColor; strokeToUse := b.currentStroke; b.mu.RUnlock()
		b.currentPath = &Path{ ID: fmt.Sprintf("path-%d", time.Now().UnixNano()), Points: []fyne.Position{adjustedPos}, Color: colorToUse, Stroke: strokeToUse }
	}
}
func (b *BoardWidget) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary && b.drawing {
		b.drawing = false
		if b.currentPath != nil && len(b.currentPath.Points) > 1 {
			b.mu.Lock(); b.paths = append(b.paths, b.currentPath); b.mu.Unlock()
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

// --- Fyne Renderer ---
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