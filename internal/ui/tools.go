package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// We need to keep track of the last used color when switching back from the eraser.
var lastSelectedColor color.Color = color.Black

// --- Custom Widget for Color Swatches ---
type colorSwatch struct {
	widget.BaseWidget
	Color    color.Color
	OnTapped func(color.Color)
}

func newColorSwatch(c color.Color, tapped func(color.Color)) *colorSwatch {
	s := &colorSwatch{Color: c, OnTapped: tapped}
	s.ExtendBaseWidget(s)
	return s
}

func (s *colorSwatch) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(s.Color)
	rect.SetMinSize(fyne.NewSize(32, 32))

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.Gray{Y: 150}
	border.StrokeWidth = 1

	return widget.NewSimpleRenderer(container.NewStack(rect, border))
}

func (s *colorSwatch) Tapped(_ *fyne.PointEvent) {
	if s.OnTapped != nil {
		s.OnTapped(s.Color)
	}
}

// --- The Main Toolbar ---
func NewToolbar(board *BoardWidget) fyne.CanvasObject {
	// toolbar with built-in tooltips
	tb := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentCreateIcon(), func() {
			board.SetColor(lastSelectedColor)
			if board.currentStroke > 10.0 {
				board.SetStroke(2.0)
			}
		}), // Pen
		widget.NewToolbarAction(theme.DeleteIcon(), func() {
			board.SetColor(color.White)
			board.SetStroke(20.0)
		}), // Eraser
	)

	// --- Color Palette ---
	onColorTapped := func(c color.Color) {
		lastSelectedColor = c
		board.SetColor(c)
	}
	colorBox := container.NewHBox(
		newColorSwatch(color.Black, onColorTapped),
		newColorSwatch(color.NRGBA{R: 255, A: 255}, onColorTapped),         // Red
		newColorSwatch(color.NRGBA{G: 255, A: 255}, onColorTapped),         // Green
		newColorSwatch(color.NRGBA{B: 255, A: 255}, onColorTapped),         // Blue
		newColorSwatch(color.NRGBA{R: 255, G: 255, A: 255}, onColorTapped), // Yellow
	)

	// --- Stroke Width Slider ---
	strokeSlider := widget.NewSlider(1.0, 50.0)
	strokeSlider.SetValue(2.0)
	strokeSlider.OnChanged = func(val float64) {
		board.SetStroke(float32(val))
	}
	sliderContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(150, 35)), strokeSlider)

	// --- Assemble everything ---
	return container.NewHBox(
		widget.NewLabel("Tool:"),
		tb,
		widget.NewSeparator(),
		widget.NewLabel("Color:"),
		colorBox,
		widget.NewSeparator(),
		widget.NewLabel("Size:"),
		sliderContainer,
		layout.NewSpacer(),
	)
}
