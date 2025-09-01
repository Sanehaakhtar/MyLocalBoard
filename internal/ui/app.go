package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

func RunApp() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Local Whiteboard")
	myWindow.Resize(fyne.NewSize(1024, 768))

	// Create the interactive board widget
	board := NewBoardWidget()

	// Create the toolbar and pass it a reference to the board
	toolbar := NewToolbar(board)

	// Set up the main layout
	content := container.NewBorder(toolbar, nil, nil, nil, board)

	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}