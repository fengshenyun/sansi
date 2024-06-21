package job

import (
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"sync"
)

var (
	once      sync.Once
	gComicJob *ComicJob
)

func init() {
	once.Do(func() {
		gComicJob = NewComicJob()
	})
}

func Start() error {
	c := cron.New()

	// 推荐banner
	if _, err := c.AddJob("@every 60s", gComicJob); err != nil {
		return errors.Wrapf(err, "add comic job failed")
	}

	c.Start()
	return nil
}
