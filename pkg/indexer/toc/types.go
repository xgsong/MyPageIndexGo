// Package toc provides TOC (Table of Contents) processing functionality.
// This package consolidates TOC detection, extraction, verification, and processing
// to reduce package sprawl in the indexer package.
package toc

// ProcessingMode represents the different TOC processing modes
type ProcessingMode string

const (
	// ModeWithPageNumbers processes TOC that has explicit page numbers
	ModeWithPageNumbers ProcessingMode = "process_toc_with_page_numbers"
	// ModeNoPageNumbers processes TOC without page numbers
	ModeNoPageNumbers ProcessingMode = "process_toc_no_page_numbers"
	// ModeNone generates structure without TOC
	ModeNone ProcessingMode = "process_no_toc"
)

// Item represents a single entry in the table of contents
type Item struct {
	Title          string `json:"title"`          // Section/chapter title
	Level          int    `json:"level"`          // Heading level (1, 2, 3, etc.)
	PhysicalIndex  int    `json:"physical_index"` // Actual page number in the document
	LogicalIndex   int    `json:"logical_index"`  // Logical page number (may differ from physical)
	HasPageNumbers bool   `json:"has_page_numbers"` // Whether this item has explicit page numbers
}

// Result represents the result of TOC processing
type Result struct {
	Items      []Item
	Mode       ProcessingMode
	Accuracy   float64
	PageNumber int // Page number where TOC was found
}

// Constants for TOC processing
const (
	MinTOCAccuracy = 0.6 // Minimum accuracy threshold for TOC
	MaxFixAttempts = 3   // Maximum attempts to fix incorrect TOC
)
