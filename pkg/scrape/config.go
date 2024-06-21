package scrape

type Config struct {
	Debug         bool
	Url           string
	Timeout       int
	RootPath      string
	MaxRetryTimes int
}
