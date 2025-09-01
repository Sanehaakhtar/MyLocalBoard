package ui

import (
	"fmt"
	"os"
	"MyLocalBoard/internal/state"
)

// Simple text-based export for now, can be enhanced to PDF later
func ExportPDF(path string, s *state.BoardState) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write a simple text representation for now
	fmt.Fprintf(file, "LocalBoard Export\n")
	fmt.Fprintf(file, "================\n\n")
	
	strokes := s.Strokes()
	fmt.Fprintf(file, "Total strokes: %d\n\n", len(strokes))
	
	for i, stroke := range strokes {
		fmt.Fprintf(file, "Stroke %d:\n", i+1)
		fmt.Fprintf(file, "  Points: %d\n", len(stroke.Points))
		fmt.Fprintf(file, "  Color: %s\n", stroke.Color)
		fmt.Fprintf(file, "  Time: %s\n", stroke.Time.Format("2006-01-02 15:04:05"))
		if len(stroke.Points) > 0 {
			fmt.Fprintf(file, "  Start: (%.2f, %.2f)\n", stroke.Points[0].X, stroke.Points[0].Y)
			if len(stroke.Points) > 1 {
				fmt.Fprintf(file, "  End: (%.2f, %.2f)\n", 
					stroke.Points[len(stroke.Points)-1].X, 
					stroke.Points[len(stroke.Points)-1].Y)
			}
		}
		fmt.Fprintf(file, "\n")
	}
	
	return nil
}