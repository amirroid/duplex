package duplex

import (
	"fmt"
)

// CalculatePlan generates a duplex printing plan for the given page count and options.
func CalculatePlan(totalPages int, opts Options) (*DuplexPlan, error) {
	if totalPages < 1 {
		return nil, fmt.Errorf("total pages must be at least 1, got %d", totalPages)
	}

	totalSheets := (totalPages + 1) / 2

	// Pass 1: Front pages (sheets 1 to totalSheets)
	frontPages := make([]int, 0, totalSheets)
	for sheet := 1; sheet <= totalSheets; sheet++ {
		frontPages = append(frontPages, 2*sheet-1)
	}

	// Pass 2: Back pages
	backPages := make([]int, 0, totalSheets)

	if opts.FlipMode == FlipModeReverse {
		// Reverse order: Sheet S down to 1
		for sheet := totalSheets; sheet >= 1; sheet-- {
			backPg := 2 * sheet
			if backPg <= totalPages {
				backPages = append(backPages, backPg)
			} else {
				// Final sheet back is blank
				if opts.OddPageMode == OddPageInsertBlank {
					backPages = append(backPages, 0) // 0 denotes blank page
				}
				// If OddPageOmit, do not add blank page (user removes sheet S)
			}
		}
	} else {
		// Forward order: Sheet 1 up to S
		for sheet := 1; sheet <= totalSheets; sheet++ {
			backPg := 2 * sheet
			if backPg <= totalPages {
				backPages = append(backPages, backPg)
			} else {
				// Final sheet back is blank
				if opts.OddPageMode == OddPageInsertBlank {
					backPages = append(backPages, 0) // 0 denotes blank page
				}
				// If OddPageOmit, do not add blank page
			}
		}
	}

	// Calculate rotation
	rotation := 0
	switch opts.Rotation {
	case Rotate180:
		rotation = 180
	case RotateNone:
		rotation = 0
	case RotateAuto:
		rotation = DetermineAutoRotation(opts.Orientation, opts.Binding)
	default:
		rotation = 0
	}

	return &DuplexPlan{
		TotalPages:   totalPages,
		TotalSheets:  totalSheets,
		FrontPages:   frontPages,
		BackPages:    backPages,
		BackRotation: rotation,
		Options:      opts,
	}, nil
}

// DetermineAutoRotation decides if 180-degree rotation is required based on
// orientation and binding edge.
func DetermineAutoRotation(orient Orientation, binding BindingEdge) int {
	if orient == OrientationLandscape {
		// Landscape documents bound on the long edge (top edge) need 180 rotation
		// when flipped top-to-bottom.
		if binding == BindingLongEdge {
			return 180
		}
		return 0
	}

	// Portrait documents bound on short edge (top edge notepad style) need 180 rotation.
	if binding == BindingShortEdge {
		return 180
	}

	// Standard book binding (portrait, long edge) needs 0 rotation.
	return 0
}
