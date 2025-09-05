package ui

import (
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func RunApp(shareLink string, board *BoardWidget) {
	myApp := app.New()
	window := myApp.NewWindow("MyLocalBoard")
	window.Resize(fyne.NewSize(1024, 768))

	if shareLink != "" {
		board.SetStatus("Share this link: " + shareLink)
	} else {
		board.SetStatus("Connecting...")
	}
	
	content := container.NewBorder(
		createToolbar(board, window),
		board.statusBar,
		nil, nil,
		board,
	)

	window.SetContent(content)
	log.Println("Starting Fyne UI...")
	window.ShowAndRun()
}

func createToolbar(board *BoardWidget, window fyne.Window) *fyne.Container {
	saveBtn := widget.NewButton("Save", func() {
		log.Println("Save button clicked")
		saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if writer == nil || err != nil { 
				log.Printf("Save dialog cancelled or error: %v", err)
				return 
			}
			log.Printf("Saving to file: %s", writer.URI().String())
			board.SaveToFile(writer)
		}, window)
		saveDialog.SetFileName("mysession.board")
		saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".board"}))
		saveDialog.Show()
	})
	
	loadBtn := widget.NewButton("Load", func() {
		log.Println("Load button clicked")
		loadDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if reader == nil || err != nil { 
				log.Printf("Load dialog cancelled or error: %v", err)
				return 
			}
			log.Printf("Loading from file: %s", reader.URI().String())
			
			// Critical fix: Run the load operation in a separate goroutine
			// to prevent blocking the UI thread
			go func() {
				board.LoadFromFile(reader)
			}()
		}, window)
		loadDialog.SetFilter(storage.NewExtensionFileFilter([]string{".board"}))
		loadDialog.Show()
	})
	
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
		widget.NewSeparator(),
		saveBtn,
		loadBtn,
	)
}