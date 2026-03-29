package progress

import (
	"github.com/schollz/progressbar/v3"
)

func NewBar(total int64, description string) *progressbar.ProgressBar {
	return progressbar.Default(total, description)
}
