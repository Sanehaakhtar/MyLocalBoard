package ui

import (
	"image/color"
	"time"

	"MyLocalBoard/internal/state"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
)

type BoardWidget struct {
	*container.Scroll
	content      *fyne.Container
	state        *state.BoardState
	tool         Tool
	path         []fyne.Position
	
	// Viewport and scrolling
	scale        float32
	showGrid     bool
	gridSize     float32
	
	// Drawing state
	isDrawing    bool
	strokeLines  []fyne.CanvasObject
}

func NewBoardWidget(s *state.BoardState) *BoardWidget {
	b := &BoardWidget{
		state:    s,
		tool:     ToolPen,
		scale:    1.0,
		gridSize: 50.0,
		showGrid: true,
	}
	
	// Create a large container for the infinite canvas
	b.content = container.NewWithoutLayout()
	b.content.Resize(fyne.NewSize(10000, 10000)) // Large canvas size
	
	// Create scroll container
	b.Scroll = container.NewScroll(b.content)
	b.Scroll.Resize(fyne.NewSize(800, 600))
	
	// Set up background and initial content
	b.setupCanvas()
	
	return b
}

func (b *BoardWidget) setupCanvas() {
	objects := []fyne.CanvasObject{}
	
	// Background
	bg := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		return color.NRGBA{R: 245, G: 246, B: 248, A: 255}
	})
	bg.Resize(fyne.NewSize(10000, 10000))
	objects = append(objects, bg)
	
	// Grid if enabled
	if b.showGrid {
		objects = append(objects, b.createGrid()...)
	}
	
	// Existing strokes
	for _, stroke := range b.state.Strokes() {
		objects = append(objects, b.strokeToLines(stroke))
	}
	
	b.content.Objects = objects
	b.content.Refresh()
}

func (b *BoardWidget) createGrid() []fyne.CanvasObject {
	var lines []fyne.CanvasObject
	gridColor := color.NRGBA{R: 220, G: 220, B: 220, A: 100}
	
	// Vertical lines
	for x := float32(0); x < 10000; x += b.gridSize {
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(x, 0)
		line.Position2 = fyne.NewPos(x, 10000)
		line.StrokeWidth = 0.5
		lines = append(lines, line)
	}
	
	// Horizontal lines  
	for y := float32(0); y < 10000; y += b.gridSize {
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(0, y)
		line.Position2 = fyne.NewPos(10000, y)
		line.StrokeWidth = 0.5
		lines = append(lines, line)
	}
	
	return lines
}

func (b *BoardWidget) strokeToLines(stroke state.Stroke) fyne.CanvasObject {
	if len(stroke.Points) < 2 {
		return container.NewWithoutLayout()
	}
	
	lines := []fyne.CanvasObject{}
	for i := 1; i < len(stroke.Points); i++ {
		p1 := stroke.Points[i-1]
		p2 := stroke.Points[i]
		
		line := canvas.NewLine(color.NRGBA{R: 0, G: 0, B: 0, A: 255})
		line.Position1 = fyne.NewPos(p1.X, p1.Y)
		line.Position2 = fyne.NewPos(p2.X, p2.Y)
		line.StrokeWidth = 2
		lines = append(lines, line)
	}
	
	return container.NewWithoutLayout(lines...)
}

// Implement desktop.Mouseable interface
func (b *BoardWidget) MouseDown(ev *desktop.MouseEvent) {
	if b.tool == ToolPen {
		// Get position relative to content
		contentPos := b.getContentPosition(ev.Position)
		b.path = []fyne.Position{contentPos}
		b.isDrawing = true
		b.strokeLines = []fyne.CanvasObject{}
	}
}

func (b *BoardWidget) MouseUp(ev *desktop.MouseEvent) {
	if b.tool == ToolPen && b.isDrawing && len(b.path) >= 2 {
		// Finalize the stroke
		points := make([]state.Point, 0, len(b.path))
		for _, p := range b.path {
			points = append(points, state.Point{X: float32(p.X), Y: float32(p.Y)})
		}
		
		op := state.NewStrokeOp(points, time.Now())
		state.EmitLocal(op)
		
		b.path = nil
		b.isDrawing = false
		b.strokeLines = nil
		b.Refresh()
	}
}

func (b *BoardWidget) MouseIn(ev *desktop.MouseEvent) {}
func (b *BoardWidget) MouseOut() {}

func (b *BoardWidget) MouseMoved(ev *desktop.MouseEvent) {
	if b.tool == ToolPen && b.isDrawing {
		contentPos := b.getContentPosition(ev.Position)
		b.path = append(b.path, contentPos)
		b.drawPreviewLine(contentPos)
	}
}

func (b *BoardWidget) getContentPosition(screenPos fyne.Position) fyne.Position {
	// Convert screen position to content position accounting for scroll
	scrollOffset := b.Scroll.Offset
	return fyne.NewPos(
		screenPos.X + scrollOffset.X,
		screenPos.Y + scrollOffset.Y,
	)
}

func (b *BoardWidget) drawPreviewLine(newPos fyne.Position) {
	if len(b.path) < 2 {
		return
	}
	
	// Draw line from previous point to current point
	prevPos := b.path[len(b.path)-2]
	line := canvas.NewLine(color.NRGBA{R: 0, G: 0, B: 0, A: 128}) // Semi-transparent for preview
	line.Position1 = prevPos
	line.Position2 = newPos
	line.StrokeWidth = 2
	
	b.strokeLines = append(b.strokeLines, line)
	b.content.Add(line)
	b.content.Refresh()
}

func (b *BoardWidget) Refresh() {
	b.setupCanvas()
}

// Implement desktop.Scrollable interface for zoom
func (b *BoardWidget) Scrolled(ev *fyne.ScrollEvent) {
	// Handle zoom with mouse wheel
	if ev.Scrolled.DY > 0 {
		b.ZoomIn()
	} else {
		b.ZoomOut()
	}
}

func (b *BoardWidget) ZoomIn() {
	oldScale := b.scale
	b.scale *= 1.2
	if b.scale > 3.0 {
		b.scale = 3.0
	}
	b.applyZoom(oldScale)
}

func (b *BoardWidget) ZoomOut() {
	oldScale := b.scale  
	b.scale /= 1.2
	if b.scale < 0.3 {
		b.scale = 0.3
	}
	b.applyZoom(oldScale)
}

func (b *BoardWidget) applyZoom(oldScale float32) {
	// Scale the content size
	newSize := fyne.NewSize(10000*b.scale, 10000*b.scale)
	b.content.Resize(newSize)
	
	// Update grid size for zoom
	b.gridSize = 50.0 * b.scale
	
	b.Refresh()
}

func (b *BoardWidget) ResetView() {
	b.scale = 1.0
	b.content.Resize(fyne.NewSize(10000, 10000))
	b.gridSize = 50.0
	b.Scroll.ScrollToTop()
	// ScrollToLeading doesn't exist, so we'll set offset manually
	b.Scroll.Offset = fyne.NewPos(0, 0)
	b.Refresh()
}

func (b *BoardWidget) ToggleGrid() {
	b.showGrid = !b.showGrid
	b.Refresh()
}

func (b *BoardWidget) SetTool(tool Tool) {
	b.tool = tool
}