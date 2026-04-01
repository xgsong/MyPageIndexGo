package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatPageRange_SinglePage(t *testing.T) {
	result := formatPageRange(5, 5)
	assert.Equal(t, "page 5", result)
}

func TestFormatPageRange_MultiplePages(t *testing.T) {
	result := formatPageRange(1, 5)
	assert.Equal(t, "pages 1-5", result)
}

func TestFormatPageRange_SamePage(t *testing.T) {
	result := formatPageRange(10, 10)
	assert.Equal(t, "page 10", result)
}

func TestFormatPageRange_LargeRange(t *testing.T) {
	result := formatPageRange(100, 150)
	assert.Equal(t, "pages 100-150", result)
}
