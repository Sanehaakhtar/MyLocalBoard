package ui

type Tool int

const (
	ToolPen Tool = iota
	ToolPan
	ToolEraser // Now enabled
	ToolText   // TODO: implement
	ToolShape  // TODO: implement
)

func (t Tool) String() string {
	switch t {
	case ToolPen:
		return "Pen"
	case ToolPan:
		return "Pan"
	case ToolEraser:
		return "Eraser"
	case ToolText:
		return "Text"
	case ToolShape:
		return "Shape"
	default:
		return "Unknown"
	}
}