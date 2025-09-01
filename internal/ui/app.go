package ui

import (
	"image/color"
	"log"

	"MyLocalBoard/internal/state"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"fyne.io/fyne/v2/theme"
)

type AppConfig struct {
	DisplayName string
	State       *state.BoardState
	Transport   TransportLike
}

type TransportLike interface {
	PeersSummary() string
	SaveSession(path string) error
	LoadSession(path string) error
}

var invalidate func()

func InvalidateCanvas() {
	if invalidate != nil {
		invalidate()
	}
}

func RunApp(cfg AppConfig) {
	a := app.New()
	w := a.NewWindow("LocalBoard - Infinite Whiteboard")
	w.Resize(fyne.NewSize(1200, 800))

	board := NewBoardWidget(cfg.State)
	invalidate = board.Refresh

	// Drawing tools
	penBtn := widget.NewButton("Pen", func() {
		board.SetTool(ToolPen)
		log.Println("Switched to Pen tool")
	})
	penBtn.Importance = widget.HighImportance

	panBtn := widget.NewButton("Pan", func() {
		board.SetTool(ToolPan) 
		log.Println("Switched to Pan tool - Use scroll bars to move around")
	})

	// View controls
	zoomInBtn := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
		board.ZoomIn()
	})
	zoomOutBtn := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
		board.ZoomOut()
	})
	resetViewBtn := widget.NewButton("Reset View", func() {
		board.ResetView()
	})
	toggleGridBtn := widget.NewButton("Toggle Grid", func() {
		board.ToggleGrid()
	})

	// Tool selection toolbar
	toolsContainer := container.NewHBox(
		widget.NewLabel("Tools:"),
		penBtn,
		panBtn,
		widget.NewSeparator(),
		widget.NewLabel("View:"),
		zoomInBtn,
		zoomOutBtn,
		resetViewBtn,
		toggleGridBtn,
	)

	// Connection and file controls
	status := widget.NewLabel("Ready - Use mouse wheel to zoom, right-click drag to pan")
	
	peerBtn := widget.NewButton("Peers", func() {
		dialog.ShowInformation("Connected Peers", cfg.Transport.PeersSummary(), w)
	})

	saveBtn := widget.NewButton("Save Session", func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			if err := cfg.Transport.SaveSession(uc.URI().Path()); err != nil {
				dialog.ShowError(err, w)
			} else {
				status.SetText("Session saved successfully")
			}
			_ = uc.Close()
		}, w)
		fd.SetFileName("session.lboard")
		fd.Show()
	})

	loadBtn := widget.NewButton("Load Session", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			if err := cfg.Transport.LoadSession(r.URI().Path()); err != nil {
				log.Println("Load error:", err)
				dialog.ShowError(err, w)
			} else {
				board.Refresh()
				status.SetText("Session loaded successfully")
			}
			_ = r.Close()
		}, w)
		fd.Show()
	})

	exportBtn := widget.NewButton("Export", func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			if err := ExportPDF(uc.URI().Path(), cfg.State); err != nil {
				dialog.ShowError(err, w)
			} else {
				status.SetText("Whiteboard exported successfully")
			}
			_ = uc.Close()
		}, w)
		fd.SetFileName("whiteboard_export.txt")
		fd.Show()
	})

	// File and network controls
	fileContainer := container.NewHBox(
		peerBtn,
		widget.NewSeparator(),
		saveBtn,
		loadBtn,
		exportBtn,
	)

	// Main toolbar combining tools and file controls
	toolbar := container.NewBorder(nil, nil, toolsContainer, fileContainer)

	// Instructions panel (collapsible)
	instructions := widget.NewLabel(`Instructions:
• Pen Tool: Click and drag to draw
• Pan Tool: Click and drag to move around the infinite canvas
• Mouse Wheel: Zoom in/out
• Grid: Toggle to show/hide the background grid
• Reset View: Return to center with 1:1 zoom`)
	instructions.Wrapping = fyne.TextWrapWord

	instructionCard := widget.NewCard("Quick Help", "", instructions)
	instructionCard.Hide() // Start hidden

	helpBtn := widget.NewButton("Help", func() {
		if instructionCard.Visible() {
			instructionCard.Hide()
		} else {
			instructionCard.Show()
		}
	})

	// Status bar
	statusContainer := container.NewHBox(
		status,
		widget.NewSeparator(),
		helpBtn,
	)

	// Main layout
	topSection := container.NewVBox(toolbar, instructionCard)
	
	bg := canvas.NewRectangle(color.NRGBA{R: 25, G: 28, B: 34, A: 255})
	mainContent := container.NewMax(bg, board)

	content := container.NewBorder(
		topSection,      // top
		statusContainer, // bottom
		nil,            // left
		nil,            // right
		mainContent,    // center
	)

	w.SetContent(content)

	// Keyboard shortcuts
	w.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeySpace:
			if board.tool == ToolPan {
				board.tool = ToolPen
				status.SetText("Switched to Pen tool")
			} else {
				board.tool = ToolPan
				status.SetText("Switched to Pan tool - Click and drag to move")
			}
		case fyne.KeyEqual: // + key for zoom in
			board.ZoomIn()
		case fyne.KeyMinus: // - key for zoom out
			board.ZoomOut()
		case fyne.KeyR: // R key to reset view
			board.ResetView()
		case fyne.KeyG: // G key to toggle grid
			board.ToggleGrid()
		}
	})

	w.ShowAndRun()
}