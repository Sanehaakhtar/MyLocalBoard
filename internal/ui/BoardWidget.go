package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Path defines a single continuous stroke on the canvas.
type Path struct {
	Points []fyne.Position
	Color  color.Color
	Stroke float32
}

// BoardWidget is our custom widget for the whiteboard canvas.
type BoardWidget struct {
	widget.BaseWidget

	paths       []*Path
	currentPath *Path
	// panX and panY track the canvas offset for the "infinite" scrolling effect.
	panX, panY float32
	drawing    bool

	// These hold the current state selected from the toolbar.
	currentColor  color.Color
	currentStroke float32
}

// NewBoardWidget creates a new instance of our whiteboard.
func NewBoardWidget() *BoardWidget {
	b := &BoardWidget{
		paths:         make([]*Path, 0),
		currentColor:  color.Black,
		currentStroke: 2.0,
	}
	b.ExtendBaseWidget(b)
	return b
}


// SetColor allows the toolbar to change the current drawing color.
func (b *BoardWidget) SetColor(c color.Color) {
	b.currentColor = c
}

// SetStroke allows changing the brush size.
func (b *BoardWidget) SetStroke(s float32) {
	b.currentStroke = s
}

// --- Fyne Custom Widget Implementation ---

// CreateRenderer is a mandatory part of the Widget interface.
func (b *BoardWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &boardWidgetRenderer{board: b}
	// A background rectangle ensures the widget is not transparent and can be interacted with.
	r.background = canvas.NewRectangle(color.White)
	r.objects = []fyne.CanvasObject{r.background}
	return r
}

// --- Fyne Desktop Events for Mouse Interaction ---

// MouseDown is called when a mouse button is pressed.
// We use the Primary button for drawing.
func (b *BoardWidget) MouseDown(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.drawing = true
		// Adjust the starting point by the current canvas pan/offset.
		adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)

		// Start a new path.
		b.currentPath = &Path{
			Points: []fyne.Position{adjustedPos},
			Color:  b.currentColor,
			Stroke: b.currentStroke,
		}
		b.paths = append(b.paths, b.currentPath)
	}
}

// MouseUp is called when a mouse button is released.
func (b *BoardWidget) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonPrimary {
		b.drawing = false
		b.currentPath = nil // Finalize the path.
	}
}

// --- Fyne Drag Events for Drawing and Panning ---

// Dragged is called when the mouse is moved while a button is held down.
func (b *BoardWidget) Dragged(e *fyne.DragEvent) {
	if b.drawing {
		// If in drawing mode, add the new point to the current path.
		adjustedPos := fyne.NewPos(e.Position.X-b.panX, e.Position.Y-b.panY)
		b.currentPath.Points = append(b.currentPath.Points, adjustedPos)
		b.Refresh()
	} else {
		// If not drawing, we must be panning. Update the canvas offset.
		b.panX += e.Dragged.DX
		b.panY += e.Dragged.DY
		b.Refresh()
	}
}

// DragEnd is called when a drag event finishes.
func (b *BoardWidget) DragEnd() {
	// This can be used to finalize any drawing logic if needed.
}

// Scrolled is another way to pan the canvas, using the mouse wheel.
func (b *BoardWidget) Scrolled(e *fyne.ScrollEvent) {
	b.panX += e.Scrolled.DX
	b.panY += e.Scrolled.DY
	b.Refresh()
}


// --- Fyne Renderer Implementation ---
// This part handles the actual drawing of objects onto the screen.

type boardWidgetRenderer struct {
	board      *BoardWidget
	background *canvas.Rectangle
	objects    []fyne.CanvasObject
}

func (r *boardWidgetRenderer) Destroy() {}

func (r *boardWidgetRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
}

func (r *boardWidgetRenderer) MinSize() fyne.Size {
	return fyne.NewSize(300, 300) // A sensible default minimum size.
}

func (r *boardWidgetRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Refresh is called by Fyne when the widget needs to be redrawn.
func (r *boardWidgetRenderer) Refresh() {
	// Start with a clean background.
	r.objects = []fyne.CanvasObject{r.background}

	// Iterate over every path and create renderable objects for them.
	for _, p := range r.board.paths {
		// A path is a series of points. We render it as a collection of small line segments.
		// Fyne does not have a native "Polyline" object, so we simulate it.
		if len(p.Points) > 1 {
			for i := 0; i < len(p.Points)-1; i++ {
				segment := canvas.NewLine(p.Color)
				segment.StrokeWidth = p.Stroke
				// Apply the pan offset to each point before rendering.
				segment.Position1 = fyne.NewPos(p.Points[i].X+r.board.panX, p.Points[i].Y+r.board.panY)
				segment.Position2 = fyne.NewPos(p.Points[i+1].X+r.board.panX, p.Points[i+1].Y+r.board.panY)
				r.objects = append(r.objects, segment)
			}
		} else if len(p.Points) == 1 {
			// If a path has only one point (a tap), draw a small circle (a dot).
			dot := canvas.NewCircle(p.Color)
			dot.Resize(fyne.NewSize(p.Stroke, p.Stroke))
			// Center the dot on the cursor position.
			dot.Move(fyne.NewPos(p.Points[0].X+r.board.panX-p.Stroke/2, p.Points[0].Y+r.board.panY-p.Stroke/2))
			r.objects = append(r.objects, dot)
		}
	}
	

	// This tells Fyne that the renderer's objects have changed and the widget needs a repaint.
	canvas.Refresh(r.board)
}