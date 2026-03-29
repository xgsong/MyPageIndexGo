package progress

import (
	"context"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
)

type Progress struct {
	bar *progressbar.ProgressBar
}

func New(total int64, description string) *Progress {
	bar := progressbar.Default(total, description)
	return &Progress{bar: bar}
}

func (p *Progress) Add(n int) {
	p.bar.Add(n)
}

func (p *Progress) Set(current int64) {
	p.bar.Set64(current)
}

func (p *Progress) SetDescription(desc string) {
	p.bar.Describe(desc)
}

func (p *Progress) Finish() {
	p.bar.Finish()
}

type Tracker struct {
	current atomic.Int64
	total   int64
	desc    string
	cb      func(done, total int64, desc string)
}

func NewTracker(total int64, desc string, cb func(done, total int64, desc string)) *Tracker {
	t := &Tracker{
		total: total,
		desc:  desc,
		cb:    cb,
	}
	t.current.Store(0)
	return t
}

func (t *Tracker) Add(n int) {
	newVal := t.current.Add(int64(n))
	if t.cb != nil {
		t.cb(newVal, t.total, t.desc)
	}
}

func (t *Tracker) Done() {
	if t.cb != nil {
		t.cb(t.total, t.total, t.desc)
	}
}

func (t *Tracker) Current() int64 {
	return t.current.Load()
}

type Stage struct {
	Name        string
	Start       int64
	End         int64
	Tracker     *Tracker
	description string
}

type MultiStageTracker struct {
	stages     []*Stage
	currentIdx int
	bar        *progressbar.ProgressBar
	total      int64
	current    atomic.Int64
}

func NewMultiStageTracker(stages []*Stage, bar *progressbar.ProgressBar) *MultiStageTracker {
	var total int64 = 0
	for _, stage := range stages {
		total += stage.End - stage.Start
	}

	mst := &MultiStageTracker{
		stages: stages,
		bar:    bar,
		total:  total,
	}
	mst.current.Store(0)

	if len(stages) > 0 {
		mst.currentIdx = 0
		stages[0].Tracker = NewTracker(
			stages[0].End-stages[0].Start,
			stages[0].Name,
			mst.callback,
		)
	}

	return mst
}

func (mst *MultiStageTracker) callback(done int64, total int64, desc string) {
	if mst.currentIdx >= len(mst.stages) {
		return
	}

	stage := mst.stages[mst.currentIdx]
	stageProgress := float64(done) / float64(total)
	stageRange := stage.End - stage.Start
	currentVal := stage.Start + int64(stageProgress*float64(stageRange))

	mst.bar.Set64(currentVal)
	mst.bar.Describe(desc)
}

func (mst *MultiStageTracker) NextStage() {
	mst.currentIdx++
	if mst.currentIdx < len(mst.stages) {
		stage := mst.stages[mst.currentIdx]
		stage.Tracker = NewTracker(
			stage.End-stage.Start,
			stage.Name,
			mst.callback,
		)
	}
}

func (mst *MultiStageTracker) CurrentTracker() *Tracker {
	if mst.currentIdx >= len(mst.stages) {
		return nil
	}
	return mst.stages[mst.currentIdx].Tracker
}

func (mst *MultiStageTracker) Finish() {
	if mst.bar != nil {
		mst.bar.Finish()
	}
}

func CtxWithProgress[T any](ctx context.Context, items []T, desc string, processor func(context.Context, T, *Tracker) error, concurrent int) error {
	if len(items) == 0 {
		return nil
	}

	tracker := NewTracker(int64(len(items)), desc, nil)

	sem := make(chan struct{}, concurrent)
	errCh := make(chan error, len(items))

	for i := range items {
		go func(idx int) {
			sem <- struct{}{}
			defer func() { <-sem }()

			err := processor(ctx, items[idx], tracker)
			errCh <- err
		}(i)
	}

	for i := 0; i < len(items); i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}

	return nil
}
