package mcp

// GenerateIndexRequest represents the input for generate_index tool
type GenerateIndexRequest struct {
	FilePath          string  `json:"file_path"`
	FileType          *string `json:"file_type,omitempty"`
	OutputPath        *string `json:"output_path,omitempty"`
	Model             *string `json:"model,omitempty"`
	MaxConcurrency    *int    `json:"max_concurrency,omitempty"`
	GenerateSummaries *bool   `json:"generate_summaries,omitempty"`
}

// GenerateIndexResponse represents the output for generate_index tool
type GenerateIndexResponse struct {
	Success   bool       `json:"success"`
	IndexPath string     `json:"index_path"`
	Stats     IndexStats `json:"stats"`
}

// SearchIndexRequest represents the input for search_index tool
type SearchIndexRequest struct {
	IndexPath  string  `json:"index_path"`
	Query      string  `json:"query"`
	OutputPath *string `json:"output_path,omitempty"`
	Model      *string `json:"model,omitempty"`
}

// SearchIndexResponse represents the output for search_index tool
type SearchIndexResponse struct {
	Success         bool             `json:"success"`
	Query           string           `json:"query"`
	Answer          string           `json:"answer"`
	ReferencedNodes []ReferencedNode `json:"referenced_nodes"`
	SearchTime      float64          `json:"search_time_seconds"`
}

// ReferencedNode represents a node referenced in search results
type ReferencedNode struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

// UpdateIndexRequest represents the input for update_index tool
type UpdateIndexRequest struct {
	ExistingIndexPath string  `json:"existing_index_path"`
	NewFilePath       string  `json:"new_file_path"`
	NewFileType       *string `json:"new_file_type,omitempty"`
	OutputPath        *string `json:"output_path,omitempty"`
	Model             *string `json:"model,omitempty"`
	MaxConcurrency    *int    `json:"max_concurrency,omitempty"`
}

// UpdateIndexResponse represents the output for update_index tool
type UpdateIndexResponse struct {
	Success    bool       `json:"success"`
	OutputPath string     `json:"output_path"`
	Stats      MergeStats `json:"stats"`
}

// IndexStats represents statistics about generated index
type IndexStats struct {
	TotalPages  int     `json:"total_pages"`
	TotalNodes  int     `json:"total_nodes"`
	TimeSeconds float64 `json:"time_seconds"`
}

// MergeStats represents statistics about merged index
type MergeStats struct {
	OriginalPages int     `json:"original_pages"`
	NewPages      int     `json:"new_pages"`
	TotalPages    int     `json:"total_pages"`
	TotalNodes    int     `json:"total_nodes"`
	TimeSeconds   float64 `json:"time_seconds"`
}
