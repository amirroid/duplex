package duplex

import (
	"reflect"
	"testing"
)

func TestCalculatePlan_EvenPages(t *testing.T) {
	tests := []struct {
		name       string
		pages      int
		flipMode   FlipMode
		wantSheets int
		wantFront  []int
		wantBack   []int
	}{
		{
			name:       "2 pages - Reverse Flip",
			pages:      2,
			flipMode:   FlipModeReverse,
			wantSheets: 1,
			wantFront:  []int{1},
			wantBack:   []int{2},
		},
		{
			name:       "2 pages - Forward Flip",
			pages:      2,
			flipMode:   FlipModeForward,
			wantSheets: 1,
			wantFront:  []int{1},
			wantBack:   []int{2},
		},
		{
			name:       "4 pages - Reverse Flip",
			pages:      4,
			flipMode:   FlipModeReverse,
			wantSheets: 2,
			wantFront:  []int{1, 3},
			wantBack:   []int{4, 2},
		},
		{
			name:       "4 pages - Forward Flip",
			pages:      4,
			flipMode:   FlipModeForward,
			wantSheets: 2,
			wantFront:  []int{1, 3},
			wantBack:   []int{2, 4},
		},
		{
			name:       "6 pages - Reverse Flip",
			pages:      6,
			flipMode:   FlipModeReverse,
			wantSheets: 3,
			wantFront:  []int{1, 3, 5},
			wantBack:   []int{6, 4, 2},
		},
		{
			name:       "8 pages - Reverse Flip (Example from Prompt)",
			pages:      8,
			flipMode:   FlipModeReverse,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{8, 6, 4, 2},
		},
		{
			name:       "8 pages - Forward Flip",
			pages:      8,
			flipMode:   FlipModeForward,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{2, 4, 6, 8},
		},
		{
			name:       "10 pages - Reverse Flip",
			pages:      10,
			flipMode:   FlipModeReverse,
			wantSheets: 5,
			wantFront:  []int{1, 3, 5, 7, 9},
			wantBack:   []int{10, 8, 6, 4, 2},
		},
		{
			name:       "10 pages - Forward Flip",
			pages:      10,
			flipMode:   FlipModeForward,
			wantSheets: 5,
			wantFront:  []int{1, 3, 5, 7, 9},
			wantBack:   []int{2, 4, 6, 8, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Orientation: OrientationPortrait,
				Binding:     BindingLongEdge,
				FlipMode:    tt.flipMode,
				Rotation:    RotateNone,
				OddPageMode: OddPageInsertBlank,
			}
			plan, err := CalculatePlan(tt.pages, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.TotalSheets != tt.wantSheets {
				t.Errorf("TotalSheets = %d, want %d", plan.TotalSheets, tt.wantSheets)
			}
			if !reflect.DeepEqual(plan.FrontPages, tt.wantFront) {
				t.Errorf("FrontPages = %v, want %v", plan.FrontPages, tt.wantFront)
			}
			if !reflect.DeepEqual(plan.BackPages, tt.wantBack) {
				t.Errorf("BackPages = %v, want %v", plan.BackPages, tt.wantBack)
			}
		})
	}
}

func TestCalculatePlan_OddPages_InsertBlank(t *testing.T) {
	tests := []struct {
		name       string
		pages      int
		flipMode   FlipMode
		wantSheets int
		wantFront  []int
		wantBack   []int
	}{
		{
			name:       "1 page - Reverse Flip",
			pages:      1,
			flipMode:   FlipModeReverse,
			wantSheets: 1,
			wantFront:  []int{1},
			wantBack:   []int{0}, // Sheet 1 back is blank
		},
		{
			name:       "1 page - Forward Flip",
			pages:      1,
			flipMode:   FlipModeForward,
			wantSheets: 1,
			wantFront:  []int{1},
			wantBack:   []int{0},
		},
		{
			name:       "3 pages - Reverse Flip",
			pages:      3,
			flipMode:   FlipModeReverse,
			wantSheets: 2,
			wantFront:  []int{1, 3},
			wantBack:   []int{0, 2}, // Sheet 2 back (blank) fed first, then sheet 1 back (2)
		},
		{
			name:       "3 pages - Forward Flip",
			pages:      3,
			flipMode:   FlipModeForward,
			wantSheets: 2,
			wantFront:  []int{1, 3},
			wantBack:   []int{2, 0},
		},
		{
			name:       "5 pages - Reverse Flip",
			pages:      5,
			flipMode:   FlipModeReverse,
			wantSheets: 3,
			wantFront:  []int{1, 3, 5},
			wantBack:   []int{0, 4, 2},
		},
		{
			name:       "5 pages - Forward Flip",
			pages:      5,
			flipMode:   FlipModeForward,
			wantSheets: 3,
			wantFront:  []int{1, 3, 5},
			wantBack:   []int{2, 4, 0},
		},
		{
			name:       "7 pages - Reverse Flip",
			pages:      7,
			flipMode:   FlipModeReverse,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{0, 6, 4, 2},
		},
		{
			name:       "7 pages - Forward Flip",
			pages:      7,
			flipMode:   FlipModeForward,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{2, 4, 6, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Orientation: OrientationPortrait,
				Binding:     BindingLongEdge,
				FlipMode:    tt.flipMode,
				Rotation:    RotateNone,
				OddPageMode: OddPageInsertBlank,
			}
			plan, err := CalculatePlan(tt.pages, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.TotalSheets != tt.wantSheets {
				t.Errorf("TotalSheets = %d, want %d", plan.TotalSheets, tt.wantSheets)
			}
			if !reflect.DeepEqual(plan.FrontPages, tt.wantFront) {
				t.Errorf("FrontPages = %v, want %v", plan.FrontPages, tt.wantFront)
			}
			if !reflect.DeepEqual(plan.BackPages, tt.wantBack) {
				t.Errorf("BackPages = %v, want %v", plan.BackPages, tt.wantBack)
			}
		})
	}
}

func TestCalculatePlan_OddPages_OmitBlank(t *testing.T) {
	tests := []struct {
		name       string
		pages      int
		flipMode   FlipMode
		wantSheets int
		wantFront  []int
		wantBack   []int
	}{
		{
			name:       "1 page - Omit Blank",
			pages:      1,
			flipMode:   FlipModeReverse,
			wantSheets: 1,
			wantFront:  []int{1},
			wantBack:   []int{}, // No back side needed
		},
		{
			name:       "3 pages - Reverse Flip Omit Blank",
			pages:      3,
			flipMode:   FlipModeReverse,
			wantSheets: 2,
			wantFront:  []int{1, 3},
			wantBack:   []int{2}, // Last sheet removed, only sheet 1 back printed
		},
		{
			name:       "7 pages - Reverse Flip Omit Blank",
			pages:      7,
			flipMode:   FlipModeReverse,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{6, 4, 2},
		},
		{
			name:       "7 pages - Forward Flip Omit Blank",
			pages:      7,
			flipMode:   FlipModeForward,
			wantSheets: 4,
			wantFront:  []int{1, 3, 5, 7},
			wantBack:   []int{2, 4, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Orientation: OrientationPortrait,
				Binding:     BindingLongEdge,
				FlipMode:    tt.flipMode,
				Rotation:    RotateNone,
				OddPageMode: OddPageOmit,
			}
			plan, err := CalculatePlan(tt.pages, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(plan.BackPages, tt.wantBack) {
				t.Errorf("BackPages = %v, want %v", plan.BackPages, tt.wantBack)
			}
		})
	}
}

func TestDetermineAutoRotation(t *testing.T) {
	tests := []struct {
		name     string
		orient   Orientation
		binding  BindingEdge
		wantRot  int
	}{
		{"Portrait Long Edge (Book)", OrientationPortrait, BindingLongEdge, 0},
		{"Portrait Short Edge (Notepad)", OrientationPortrait, BindingShortEdge, 180},
		{"Landscape Long Edge (Calendar)", OrientationLandscape, BindingLongEdge, 180},
		{"Landscape Short Edge (Side bind)", OrientationLandscape, BindingShortEdge, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rot := DetermineAutoRotation(tt.orient, tt.binding)
			if rot != tt.wantRot {
				t.Errorf("DetermineAutoRotation() = %d, want %d", rot, tt.wantRot)
			}
		})
	}
}

func TestCalculatePlan_Rotation(t *testing.T) {
	optsAutoPortraitShort := Options{
		Orientation: OrientationPortrait,
		Binding:     BindingShortEdge,
		FlipMode:    FlipModeReverse,
		Rotation:    RotateAuto,
		OddPageMode: OddPageInsertBlank,
	}
	plan, err := CalculatePlan(4, optsAutoPortraitShort)
	if err != nil {
		t.Fatal(err)
	}
	if plan.BackRotation != 180 {
		t.Errorf("BackRotation = %d, want 180", plan.BackRotation)
	}

	optsExplicit180 := Options{
		Orientation: OrientationPortrait,
		Binding:     BindingLongEdge,
		FlipMode:    FlipModeReverse,
		Rotation:    Rotate180,
		OddPageMode: OddPageInsertBlank,
	}
	plan2, _ := CalculatePlan(4, optsExplicit180)
	if plan2.BackRotation != 180 {
		t.Errorf("BackRotation = %d, want 180", plan2.BackRotation)
	}
}

func TestCalculatePlan_InvalidPages(t *testing.T) {
	opts := DefaultOptions(OrientationPortrait)
	_, err := CalculatePlan(0, opts)
	if err == nil {
		t.Errorf("expected error for 0 pages, got nil")
	}
	_, err = CalculatePlan(-5, opts)
	if err == nil {
		t.Errorf("expected error for negative pages, got nil")
	}
}

func TestPlanFormatting(t *testing.T) {
	opts := DefaultOptions(OrientationPortrait)
	plan, _ := CalculatePlan(7, opts)
	if plan.FrontPagesList() != "1, 3, 5, 7" {
		t.Errorf("FrontPagesList() = %q, want '1, 3, 5, 7'", plan.FrontPagesList())
	}
	if plan.BackPagesList() != "[Blank], 6, 4, 2" {
		t.Errorf("BackPagesList() = %q, want '[Blank], 6, 4, 2'", plan.BackPagesList())
	}
}
