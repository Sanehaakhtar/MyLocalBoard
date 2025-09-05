package ui

import (
	"encoding/json"
	"image/color"
	"io"
	"log"
	"sync"
	"crypto/rand"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Path struct (unchanged)
type Path struct {
	ID      string          `json:"id"`
	OwnerID string          `json:"owner_id"`
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
	LocalClientID   string
	OnNewPath       func(p Path)
	OnClear         func()
	OnSave          func() []Path
	OnLoad          func(paths []Path)
	statusBar       *widget.Label
}

var _ fyne.Widget = (*BoardWidget)(nil)
var _ fyne.Draggable = (*BoardWidget)(nil)
var _ desktop.Mouseable = (*BoardWidget)(nil)

func NewBoardWidget() *BoardWidget {
	b := &BoardWidget{
		paths:         make([]*Path, 0),
		currentColor:  "black",
		currentStroke: 3.0,
		statusBar:     widget.NewLabel("Ready"),
	}
	b.ExtendBaseWidget(b)
	return b
}

// Generate unique ID for paths
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

func (b *BoardWidget) SetLocalClientID(id string) { 
	b.LocalClientID = id 
}

func (b *BoardWidget) GetAllPathsAsValues() []Path {
	b.mu.RLock()
	defer b.mu.RUnlock()
	paths := make([]Path, 0, len(b.paths))
	for _, pathPtr := range b.paths {
		if pathPtr != nil { 
			paths = append(paths, *pathPtr) 
		}
	}
	return paths
}

func (b *BoardWidget) clearPathsByOwner(ownerID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if ownerID == "all" {
		b.paths = make([]*Path, 0)
	} else {
		filteredPaths := make([]*Path, 0)
		for _, path := range b.paths {
			if path.OwnerID != ownerID { 
				filteredPaths = append(filteredPaths, path) 
			}
		}
		b.paths = filteredPaths
	}
	b.Refresh()
}

// Thread-safe UI update methods
func (b *BoardWidget) AddRemotePath(p Path) {
	b.mu.Lock()
	pathCopy := p // Make a copy
	b.paths = append(b.paths, &pathCopy)
	b.mu.Unlock()
	b.Refresh()
}

func (b *BoardWidget) ClearRemote(ownerID string) {
	b.clearPathsByOwner(ownerID)
}

func (b *BoardWidget) SetStatus(text string) {
	// Use a goroutine to safely update status from any thread
	go func() {
		b.statusBar.SetText(text)
	}()
}

// ClearPaths is called by a local UI button click
func (b *BoardWidget) ClearPaths() {
	if b.OnClear != nil { 
		b.OnClear() 
	}
}

func (b *BoardWidget) SaveToFile(writer fyne.URIWriteCloser) {
	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("Error closing writer: %v", err)
		}
	}()
	
	log.Println("SaveToFile: Starting save operation")
	
	if b.OnSave == nil { 
		b.SetStatus("Save function not available")
		log.Println("SaveToFile: OnSave callback is nil")
		return 
	}
	
	pathsToSave := b.OnSave()
	log.Printf("SaveToFile: Got %d paths to save", len(pathsToSave))
	
	jsonData, err := json.MarshalIndent(pathsToSave, "", "  ")
	if err != nil { 
		log.Printf("SaveToFile: Error marshaling: %v", err)
		b.SetStatus("Error saving file")
		return 
	}
	
	if _, err := writer.Write(jsonData); err != nil { 
		log.Printf("SaveToFile: Error writing: %v", err)
		b.SetStatus("Error writing file")
	} else {
		b.SetStatus(fmt.Sprintf("Saved %d drawings", len(pathsToSave)))
		log.Printf("SaveToFile: Successfully saved %d paths", len(pathsToSave))
	}
}

func (b *BoardWidget) LoadFromFile(reader fyne.URIReadCloser) {
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("Error closing reader: %v", err)
		}
	}()
	
	log.Println("LoadFromFile: Starting load operation")
	b.SetStatus("Loading file...")
	
	if b.OnLoad == nil { 
		log.Println("LoadFromFile: OnLoad callback is nil")
		b.SetStatus("Load function not available")
		return 
	}
	
	// Read all data from file
	jsonData, err := io.ReadAll(reader)
	if err != nil { 
		log.Printf("LoadFromFile: Error reading file: %v", err)
		b.SetStatus("Error reading file")
		return 
	}
	
	log.Printf("LoadFromFile: Read %d bytes from file", len(jsonData))
	
	// Parse JSON
	var loadedPaths []Path
	if err := json.Unmarshal(jsonData, &loadedPaths); err != nil { 
		log.Printf("LoadFromFile: Error unmarshaling JSON: %v", err)
		b.SetStatus("Error parsing file - invalid format")
		return 
	}
	
	log.Printf("LoadFromFile: Successfully parsed %d paths from file", len(loadedPaths))
	
	// Clear current paths and add loaded ones
	b.mu.Lock()
	b.paths = make([]*Path, 0, len(loadedPaths))
	for _, path := range loadedPaths {
		pathCopy := path
		b.paths = append(b.paths, &pathCopy)
	}
	b.mu.Unlock()
	
	// Refresh the UI
	b.Refresh()
	
	// Update status
	b.SetStatus(fmt.Sprintf("Loaded %d drawings", len(loadedPaths)))
	log.Printf("LoadFromFile: Load operation completed successfully")
	
	// Call network sync callback if needed
	if b.OnLoad != nil {
		b.OnLoad(loadedPaths)
	}
}

// Convert color.Color to string representation
func colorToString(c color.Color) string {
	r, g, b, _ := c.RGBA()
	if r == 65535 && g == 0 && b == 0 {
		return "red"
	} else if r == 0 && g == 0 && b == 65535 {
		return "blue"
	} else if r == 0 && g == 65535 && b == 0 {
		return "green"
	}
	return "black"
}

func (b *BoardWidget) SetColor(c color.Color) { 
	b.currentColor = colorToString(c)
}

func (b *BoardWidget) SetStroke(s float32) { 
	b.currentStroke = s 
}

func (b *BoardWidget) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.drawing = true
		adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.currentPath = &Path{
			ID:      generateID(),
			OwnerID: b.LocalClientID,
			Points:  []fyne.Position{adjustedPos},
			Color:   b.currentColor,
			Stroke:  b.currentStroke,
		}
		b.Refresh()
	}
}

func (b *BoardWidget) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary && b.drawing {
		b.drawing = false
		if b.currentPath != nil && len(b.currentPath.Points) > 1 {
			if b.OnNewPath != nil { 
				b.OnNewPath(*b.currentPath) 
			}
		}
		b.currentPath = nil
		b.Refresh()
	}
}

func (b *BoardWidget) Dragged(e *fyne.DragEvent) {
	if b.drawing && b.currentPath != nil {
		adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.currentPath.Points = append(b.currentPath.Points, adjustedPos)
		b.Refresh()
	} else if !b.drawing {
		b.panX += e.Dragged.DX
		b.panY += e.Dragged.DY
		b.Refresh()
	}
}

func (b *BoardWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &boardWidgetRenderer{board: b}
	r.background = canvas.NewRectangle(color.White)
	return r
}

type boardWidgetRenderer struct { 
	board      *BoardWidget
	background *canvas.Rectangle 
}

func (r *boardWidgetRenderer) Objects() []fyne.CanvasObject {
    r.board.mu.RLock()
    defer r.board.mu.RUnlock()
    
    objects := []fyne.CanvasObject{r.background}
    pathsToRender := make([]*Path, len(r.board.paths))
    copy(pathsToRender, r.board.paths)
    
    if r.board.drawing && r.board.currentPath != nil { 
    	pathsToRender = append(pathsToRender, r.board.currentPath) 
    }
    
    for _, p := range pathsToRender {
        if p == nil {
            continue
        }
        
        var pathColor color.Color = color.Black
        if p.Color == "red" { 
        	pathColor = color.RGBA{R: 255, A: 255}
        } else if p.Color == "blue" { 
        	pathColor = color.RGBA{B: 255, A: 255}
        } else if p.Color == "green" { 
        	pathColor = color.RGBA{G: 255, A: 255} 
        }
        
        if len(p.Points) > 1 {
            for i := 0; i < len(p.Points)-1; i++ {
                segment := canvas.NewLine(pathColor)
                segment.StrokeWidth = p.Stroke
                segment.Position1 = fyne.NewPos(p.Points[i].X+r.board.panX, p.Points[i].Y+r.board.panY)
                segment.Position2 = fyne.NewPos(p.Points[i+1].X+r.board.panX, p.Points[i+1].Y+r.board.panY)
                objects = append(objects, segment)
            }
        }
    }
    return objects
}

func (r *boardWidgetRenderer) Refresh() { 
	canvas.Refresh(r.board) 
}

func (b *BoardWidget) MouseIn(*desktop.MouseEvent) {}
func (b *BoardWidget) MouseOut() {}
func (b *BoardWidget) MouseMoved(*desktop.MouseEvent) {}
func (b *BoardWidget) DragEnd() {}
func (r *boardWidgetRenderer) Destroy() {}
func (r *boardWidgetRenderer) Layout(size fyne.Size) { 
	r.background.Resize(size) 
}
func (r *boardWidgetRenderer) MinSize() fyne.Size { 
	return fyne.NewSize(300, 300) 
}
func (b *BoardWidget) Scrolled(e *fyne.ScrollEvent) { 
	b.panX += e.Scrolled.DX
	b.panY += e.Scrolled.DY
	b.Refresh() 
}