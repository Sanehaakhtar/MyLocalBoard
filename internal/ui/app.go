package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// This version of RunApp does NOT take a BoardWidget argument.
func RunApp(shareLink string, board *BoardWidget) { // The board argument is here but we will ignore it for now
	myApp := app.New()
	myWindow := myApp.NewWindow("Local Whiteboard")
	myWindow.Resize(fyne.NewSize(1024, 768))

	// We create a new, blank board every time.
	localBoard := NewBoardWidget()
	toolbar := NewToolbar(localBoard)

	var topContent fyne.CanvasObject
	if shareLink != "" {
		// We are the HOST
		linkEntry := widget.NewEntry()
		linkEntry.SetText(shareLink)
		linkEntry.Disable()

		copyButton := widget.NewButton("Copy", func() {
			myWindow.Clipboard().SetContent(shareLink)
		})
		topContent = container.NewBorder(nil, nil, nil, copyButton, linkEntry)
	} else {
        // We are a CLIENT
        // This message is our confirmation on the Client side!
        topContent = widget.NewLabel("Connection successful! You can now draw.")
    }

	content := container.NewBorder(
		container.NewVBox(topContent, toolbar),
		nil, nil, nil, localBoard,
	)

	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}