package scrape

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
)

var (
	SupportImageSuffix = map[string]bool{
		"jpg":  true,
		"jpeg": true,
		"png":  true,
		"webp": true,
	}
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.DebugLevel)
}

const (
	DefaultTimeout       = 10 // 秒
	DefaultMetadataPath  = "./data/meta"
	DefaultImageDataPath = "./data/images"
	DefaultPageDataPath  = "./data/pages"
)

type Comics struct {
	Debug          bool
	MetadataPath   string
	ImageDataPath  string
	Timeout        int
	Url            string
	Number         int
	Root           string
	ImagePath      string
	PagePath       string
	MetaPath       string
	Title          string
	EnTitle        string
	Desc           string
	LastModifyTime string
	CoverUrl       string
	PageUrls       []string
	ImageUrls      []string
	htmlContent    []byte
	rootDoc        *goquery.Document
	pageDocs       map[string]*goquery.Document
}

func New(url string) *Comics {
	return &Comics{
		Debug:         false,
		Url:           url,
		MetadataPath:  DefaultMetadataPath,
		ImageDataPath: DefaultImageDataPath,
		Timeout:       DefaultTimeout,
		PageUrls:      []string{},
		ImageUrls:     []string{},
	}
}

func NewWithConfig(c *Config) *Comics {
	log.Debugf("scrape config: %+v", c)

	return &Comics{
		Debug:     c.Debug,
		Url:       c.Url,
		Root:      c.RootPath,
		PageUrls:  []string{},
		ImageUrls: []string{},
	}
}

func (c *Comics) Scrape() error {
	if err := c.GetMainContent(); err != nil {
		return err
	}

	if err := c.GetMainBasicInfo(); err != nil {
		return err
	}

	if err := c.GetMainPageUrls(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) Init() error {
	logField := log.Fields{"content": "scrape-init"}

	u, err := url.ParseRequestURI(c.Url)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseImageURIFailed").Error(err)
		return err
	}

	dir, file := filepath.Split(u.Path)
	log.WithFields(logField).Debugf("dir:%v, file:%v", dir, file)

	if !strings.HasSuffix(file, ".html") {
		log.WithFields(logField).WithField("position", "NotFindSuffix").Error("no html suffix")
		return errors.New("no html suffix")
	}

}

func (c *Comics) GetMainContent() error {
	var (
		err error
	)

	c.htmlContent, err = DownloadPage(c.Url, c.Debug)
	if err != nil {
		return err
	}

	c.rootDoc, err = goquery.NewDocumentFromReader(bytes.NewReader(c.htmlContent))
	if err != nil {
		return err
	}

	return nil
}

// GetMainBasicInfo Get Comic's title, cover, desc etc.
func (c *Comics) GetMainBasicInfo() error {
	if err := c.GetTitle(); err != nil {
		return err
	}

	if err := c.GetDesc(); err != nil {
		return err
	}

	if err := c.GetCoverUrl(); err != nil {
		return err
	}

	if err := c.GetLastModifyTime(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) GetTitle() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-title a").Each(func(i int, s *goquery.Selection) {
		c.Title = s.Text()
		c.Title = strings.Trim(c.Title, " \n\t\r")
		c.EnTitle = ParseCnToEn(c.Title)
		log.WithField("content", "title").Infof("title:%v, en-title:%v", c.Title, c.EnTitle)
	})

	return nil
}

func (c *Comics) GetDesc() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .dis").Each(func(i int, s *goquery.Selection) {
		c.Desc = s.Text()
		c.Desc = strings.Trim(c.Desc, " \n\t\r")
		log.WithField("content", "desc").Infof("desc:%v", c.Desc)
	})

	return nil
}

func (c *Comics) GetCoverUrl() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .c-img img").Each(func(i int, s *goquery.Selection) {
		logField := log.Fields{"content": "cover-url"}

		src, exist := s.Attr("src")
		if !exist {
			log.WithFields(logField).Info("no src attr")
			return
		}

		c.Url = src
		log.WithFields(logField).Infof("cover url:%v", c.Url)
	})

	return nil
}

func (c *Comics) GetLastModifyTime() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-meta li").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		text = strings.Trim(text, " \n\t\r")

		logField := log.Fields{"content": "last-modify-time", "text": text}

		items := strings.Split(text, ":")
		if len(items) != 2 {
			log.WithFields(logField).Info("invalid format")
			return
		}

		if _, err := time.ParseInLocation("2006-01-02", items[1], time.Local); err != nil {
			log.WithFields(logField).Info("no include yyyy-mm-dd time")
			return
		}

		c.LastModifyTime = items[1]
		log.WithFields(logField).Debugf("lastModifyTime:%v", c.LastModifyTime)
	})

	return nil
}

func (c *Comics) GetMainPageUrls() error {
	existPages := make(map[string]bool, 8)
	pageUrls := make([]string, 0, 8)
	pageUrls = append(pageUrls, c.Url)

	c.rootDoc.Find(".container .content-wrap .content .article-content .article-paging .post-page-numbers").Each(func(i int, s *goquery.Selection) {
		logField := log.Fields{"content": "get-page-url"}

		href, exist := s.Attr("href")
		if !exist {
			log.WithFields(logField).WithField("position", "NoHrefAttr").Info("no href attr")
			return
		}

		log.WithFields(logField).Infof("i:%v,href:%v", i, href)

		logField["href"] = href
		if existPages[href] {
			log.WithFields(logField).WithField("position", "AlreadyExist").Info("already process")
			return
		}

		if !c.IsValidPageUrl(href) {
			return
		}

		pageUrls = append(pageUrls, href)
		existPages[href] = true
	})

	c.PageUrls = pageUrls
	return nil
}

func (c *Comics) GetPageUrlsContent() error {

}

func (c *Comics) GetImageUrls(pageUrls []string) error {
	for _, pageUrl := range pageUrls {
		imageUrls, err := getImageUrl(pageUrl)
		if err != nil {

		}
	}
}

func getImageUrl(pageUrl string) ([]string, error) {
	existImages := make(map[string]bool, 8)
	imageUrls := make([]string, 0, 8)

	c.rootDoc.Find(".container .content-wrap .content .article-content p img").Each(func(i int, s *goquery.Selection) {
		logField := log.Fields{"content": "get-image-url"}

		src, exist := s.Attr("src")
		if !exist {
			log.WithFields(logField).WithField("position", "NoSrcAttr").Info("no src attr")
			return
		}

		log.WithFields(logField).Infof("index:%v, src:%v", i, src)

		logField["src"] = src
		if existImages[src] {
			log.WithFields(logField).WithField("position", "AlreadyExist").Info("already process")
			return
		}

		if !c.IsValidImageUrl(src) {
			return
		}

		imageUrls = append(imageUrls, src)
		existImages[src] = true
	})

	return imageUrls, nil
}

func (c *Comics) IsValidPageUrl(pageUrl string) bool {
	var (
		pagePrefix = "page"
		pageSuffix = ".html"
		logField   = log.Fields{"content": "verify-page-url", "page-url": pageUrl}
	)

	u, err := url.ParseRequestURI(pageUrl)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseImageURIFailed").Error(err)
		return false
	}

	dir, file := filepath.Split(u.Path)
	log.WithFields(logField).Debugf("dir:%v, file:%v", dir, file)

	file = strings.ToLower(file)
	if !strings.HasPrefix(file, pagePrefix) {
		log.WithFields(logField).WithField("position", "NotFindPrefix").Errorf("no %v prefix", pagePrefix)
		return false
	}

	if !strings.HasSuffix(file, pageSuffix) {
		log.WithFields(logField).WithField("position", "NotFindSuffix").Error("no %v suffix", pageSuffix)
		return false
	}

	return true
}

func (c *Comics) IsValidImageUrl(imageUrl string) bool {
	logField := log.Fields{"content": "verify-image-url", "image-url": imageUrl}

	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseImageURIFailed").Error(err)
		return false
	}

	dir, file := filepath.Split(u.Path)
	log.WithFields(logField).Debugf("dir:%v, file:%v", dir, file)

	file = strings.ToLower(file)
	pos := strings.LastIndex(file, ".")
	if pos == -1 {
		log.WithFields(logField).WithField("position", "NotFindAnyDot").Error("invalid suffix")
		return false
	}

	suffix := file[pos+1:]
	if !SupportImageSuffix[suffix] {
		log.WithFields(logField).WithField("position", "UnsupportedImageSuffix").Errorf("suffix:%v", suffix)
		return false
	}

	return true
}
