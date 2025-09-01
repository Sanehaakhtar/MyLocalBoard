package ui

import (
	"MyLocalBoard/internal/state"

	"github.com/jung-kurt/gofpdf"
)

func ExportPDF(path string, s *state.BoardState) error {
	p := gofpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.SetDrawColor(0,0,0)
	p.SetLineWidth(0.5)

	for _, st := range s.Strokes() {
		for i:=1;i<len(st.Points);i++{
			p.Line(
				float64(st.Points[i-1].X/3), float64(st.Points[i-1].Y/3),
				float64(st.Points[i].X/3),   float64(st.Points[i].Y/3),
			)
		}
	}
	return p.OutputFileAndClose(path)
}
