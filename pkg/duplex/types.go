package duplex

import "fmt"

// Orientation represents page orientation.
type Orientation string

const (
	OrientationPortrait  Orientation = "Portrait"
	OrientationLandscape Orientation = "Landscape"
)

// BindingEdge represents how the double-sided document is bound or flipped.
type BindingEdge string

const (
	BindingLongEdge  BindingEdge = "LongEdge"  // Standard book / tablet
	BindingShortEdge BindingEdge = "ShortEdge" // Notepad / calendar
)

// FlipMode determines the order in which sheets are fed on the second pass.
type FlipMode string

const (
	// FlipModeReverse: Stack is flipped over directly; the last printed sheet is fed first.
	// Back pages are ordered in reverse (e.g., 8, 6, 4, 2 or [blank], 6, 4, 2).
	FlipModeReverse FlipMode = "Reverse"

	// FlipModeForward: Stack is re-collated or printer feeds bottom-up; sheet 1 is fed first.
	// Back pages are ordered forward (e.g., 2, 4, 6, 8 or 2, 4, 6, [blank]).
	FlipModeForward FlipMode = "Forward"
)

// RotationMode determines whether back pages should be rotated 180 degrees.
type RotationMode string

const (
	RotateAuto RotationMode = "Auto" // Automatically determined from Orientation and BindingEdge
	RotateNone RotationMode = "None" // 0 degrees
	Rotate180  RotationMode = "180"  // 180 degrees
)

// OddPageMode defines how the blank back side of an odd-numbered final sheet is handled.
type OddPageMode string

const (
	// OddPageInsertBlank inserts a blank page into the back PDF.
	// This allows the user to reload the entire paper stack without removing sheets.
	OddPageInsertBlank OddPageMode = "InsertBlank"

	// OddPageOmit omits the blank page from the back PDF.
	// The user must manually remove the last printed sheet before reloading.
	OddPageOmit OddPageMode = "Omit"
)

// Options contains configuration for generating a duplex print plan.
type Options struct {
	Orientation Orientation
	Binding     BindingEdge
	FlipMode    FlipMode
	Rotation    RotationMode
	OddPageMode OddPageMode
}

// DefaultOptions returns the most common default settings for duplex printing.
func DefaultOptions(orient Orientation) Options {
	return Options{
		Orientation: orient,
		Binding:     BindingLongEdge,
		FlipMode:    FlipModeReverse,
		Rotation:    RotateAuto,
		OddPageMode: OddPageInsertBlank,
	}
}

// DuplexPlan contains the calculated plan for duplex printing.
type DuplexPlan struct {
	TotalPages   int     // Total pages in source PDF
	TotalSheets  int     // Total physical sheets needed: ceil(TotalPages / 2)
	FrontPages   []int   // 1-indexed page numbers for pass 1 (e.g., [1, 3, 5, 7])
	BackPages    []int   // 1-indexed page numbers for pass 2 (0 indicates a blank page)
	BackRotation int     // 0 or 180 degrees
	Options      Options // The options used to compute this plan
}

// FrontPagesList returns the front pages formatted as strings (e.g., "1, 3, 5, 7").
func (p DuplexPlan) FrontPagesList() string {
	return formatPageList(p.FrontPages)
}

// BackPagesList returns the back pages formatted as strings (e.g., "[Blank], 6, 4, 2").
func (p DuplexPlan) BackPagesList() string {
	return formatPageList(p.BackPages)
}

func formatPageList(pages []int) string {
	if len(pages) == 0 {
		return "(none)"
	}
	var res string
	for i, pg := range pages {
		var s string
		if pg == 0 {
			s = "[Blank]"
		} else {
			s = fmt.Sprintf("%d", pg)
		}
		if i > 0 {
			res += ", "
		}
		res += s
	}
	return res
}
