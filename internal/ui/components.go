package ui

// Rect uses terminal cell coordinates. The right and bottom edges are excluded.
type Rect struct {
	X, Y, Width, Height int
}

func (r Rect) Contains(x, y int) bool {
	return r.Width > 0 && r.Height > 0 && x >= r.X && y >= r.Y && x-r.X < r.Width && y-r.Y < r.Height
}

type SelectOption struct {
	Label, Value string
	Disabled     bool
}

// Changed previews a focused option; only Confirmed commits the value.
type SelectResult struct {
	Changed, Confirmed, Cancelled bool
	Value                         string
}

func optionIndex(options []SelectOption, value string) int {
	for i, option := range options {
		if option.Value == value {
			return i
		}
	}
	return -1
}

func enabledOption(options []SelectOption, from, direction int) int {
	for i := from; i >= 0 && i < len(options); i += direction {
		if !options[i].Disabled {
			return i
		}
	}
	return -1
}
