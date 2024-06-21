package job

import (
	"sync/atomic"
)

type ComicJob struct {
	running int32
}

func NewComicJob() *ComicJob {
	return &ComicJob{}
}

func (job *ComicJob) Run() {
	if atomic.LoadInt32(&job.running) == 1 {
		return
	}

	atomic.SwapInt32(&job.running, 1)

	job.Process()

	atomic.SwapInt32(&job.running, 0)
}

func (job *ComicJob) Process() {
	return
}
