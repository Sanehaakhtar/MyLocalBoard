package state

import (
	"image/color" // <-- ADD THIS IMPORT
	"time"
)

type Point struct{ X, Y float32 }

type Stroke struct {
	ID     string
	Points []Point
	Color  color.Color // The color of the stroke
	Width  float32     // The width of the stroke (REMOVE THE DUPLICATE)
	Time   time.Time
}

type OpType string

const (
	OpInsertStroke OpType = "insert_stroke"
	OpDeleteStroke OpType = "delete_stroke"
)

type Op struct {
	Type    OpType
	Stroke  *Stroke
	Target  string // ID of stroke to delete
	Lamport uint64
	Site    string
}

