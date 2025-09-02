package ui

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func RunApp(shareLink string, board *BoardWidget) {
	myApp := app.New()
	window := myApp.NewWindow("MyLocalBoard")
	window.Resize(fyne.NewSize(1024, 768))

	go func() {
		for statusText := range board.statusChan {
			board.statusBar.SetText(statusText)
		}
	}()

	if shareLink != "" {
		board.SetStatus("Share this link: " + shareLink)
	} else {
		board.SetStatus("Connecting...")
	}
	
	content := container.NewBorder(
		createToolbar(board),
		board.statusBar,
		nil, nil,
		board,
	)

	window.SetContent(content)
	log.Println("Starting Fyne UI...")
	window.ShowAndRun()
}

func createToolbar(board *BoardWidget) *fyne.Container {
	return container.NewHBox(
		widget.NewLabel("Colors:"),
		widget.NewButton("Black", func() { board.SetColor(color.Black) }),
		widget.NewButton("Red", func() { board.SetColor(color.RGBA{R: 255, A: 255}) }),
		widget.NewButton("Blue", func() { board.SetColor(color.RGBA{B: 255, A: 255}) }),
		widget.NewButton("Green", func() { board.SetColor(color.RGBA{G: 255, A: 255}) }),
		widget.NewSeparator(),
		widget.NewLabel("Stroke:"),
		widget.NewButton("Thin", func() { board.SetStroke(1.0) }),
		widget.NewButton("Medium", func() { board.SetStroke(3.0) }),
		widget.NewButton("Thick", func() { board.SetStroke(6.0) }),
		widget.NewSeparator(),
		widget.NewButton("Clear My Drawings", func() { board.ClearPaths() }),
	)
}