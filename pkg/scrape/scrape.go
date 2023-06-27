package scrape

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	DefaultTimeout         = 10 // 秒
	DefaultRootPath        = "./"
	DefaultMetadataPath    = "meta"
	DefaultImageDataPath   = "images"
	DefaultPageDataPath    = "pages"
	DefaultContentDataPath = "content"
)

type Comics struct {
	Debug           bool
	RootPath        string
	Timeout         int
	MainUrl         string
	Number          int
	Title           string
	EnTitle         string
	Desc            string
	LastModifyTime  string
	CoverUrl        string
	PageUrls        []string
	ImageUrls       []string
	rootHtmlContent []byte
	rootDoc         *goquery.Document
	pageDocs        map[string]*goquery.Document
}

func New(url string) *Comics {
	return &Comics{
		Debug:     false,
		MainUrl:   url,
		RootPath:  DefaultRootPath,
		Timeout:   DefaultTimeout,
		PageUrls:  []string{},
		ImageUrls: []string{},
		pageDocs:  make(map[string]*goquery.Document),
	}
}

func NewWithConfig(cfg *Config) *Comics {
	log.Debugf("scrape config: %+v", cfg)

	c := New(cfg.Url)
	c.Debug = cfg.Debug

	if cfg.RootPath != "" {
		c.RootPath = cfg.RootPath
		//c.PageDataPath = filepath.Join(cfg.RootPath, DefaultPageDataPath)
		//c.ImageDataPath = filepath.Join(cfg.RootPath, DefaultImageDataPath)
		//c.MetadataPath = filepath.Join(cfg.RootPath, DefaultMetadataPath)
	}

	return c
}

func (c *Comics) Scrape() error {
	if err := c.Validity(); err != nil {
		return err
	}

	if err := c.GetMainContent(); err != nil {
		return err
	}

	if err := c.ParseMainBasicInfo(); err != nil {
		return err
	}

	if err := c.ParseMainPageUrls(); err != nil {
		return err
	}

	if err := c.GetPageUrlsContent(); err != nil {
		return err
	}

	if err := c.GetImageUrls(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) Validity() error {
	logField := log.Fields{"content": "validity"}

	if c.MainUrl == "" {
		log.WithFields(logField).Error("empty main url")
		return errors.New("empty main url")
	}

	u, err := url.ParseRequestURI(c.MainUrl)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseMainURIFailed").Error(err)
		return err
	}

	dir, file := filepath.Split(u.Path)
	log.WithFields(logField).Debugf("dir:%v, file:%v", dir, file)

	if !strings.HasSuffix(file, ".html") {
		log.WithFields(logField).Error("no html suffix")
		return errors.New("no html suffix")
	}

	pos := strings.LastIndex(file, ".html")
	num, err := strconv.Atoi(file[:pos])
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseNumber").Error(err)
		return errors.New("invalid number")
	}

	c.Number = num
	return nil
}

func (c *Comics) GetMainContent() error {
	var (
		err      error
		logField = log.Fields{"content": "main-content", "page-url": c.MainUrl}
	)

	c.rootHtmlContent, err = DownloadPage(c.MainUrl, c.Debug)
	if err != nil {
		log.WithFields(logField).WithField("position", "DownloadMainPageFailed").Error(err)
		return err
	}

	c.rootDoc, err = goquery.NewDocumentFromReader(bytes.NewReader(c.rootHtmlContent))
	if err != nil {
		log.WithFields(logField).WithField("position", "ReadMainPageContentFailed").Error(err)
		return err
	}

	log.WithFields(logField).Debug("download main page success")
	return nil
}

// ParseMainBasicInfo Get Comic's title, cover, desc etc.
func (c *Comics) ParseMainBasicInfo() error {
	if err := c.parseTitle(); err != nil {
		return err
	}

	if err := c.parseDesc(); err != nil {
		return err
	}

	if err := c.parseCoverUrl(); err != nil {
		return err
	}

	if err := c.parseLastModifyTime(); err != nil {
		return err
	}

	if err := c.WriteMetadata(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) parseTitle() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-title a").Each(func(i int, s *goquery.Selection) {
		c.Title = s.Text()
		c.Title = strings.Trim(c.Title, " \n\t\r")
		c.EnTitle = ParseCnToEn(c.Title)
		log.WithField("content", "title").Infof("title:%v, en-title:%v", c.Title, c.EnTitle)
	})

	return nil
}

func (c *Comics) parseDesc() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .dis").Each(func(i int, s *goquery.Selection) {
		c.Desc = s.Text()
		c.Desc = strings.Trim(c.Desc, " \n\t\r")
		log.WithField("content", "desc").Infof("desc:%v", c.Desc)
	})

	return nil
}

func (c *Comics) parseCoverUrl() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .c-img img").Each(func(i int, s *goquery.Selection) {
		logField := log.Fields{"content": "cover-url"}

		src, exist := s.Attr("src")
		if !exist {
			log.WithFields(logField).Info("no src attr")
			return
		}

		c.CoverUrl = src
		log.WithFields(logField).Infof("cover url:%v", c.CoverUrl)
	})

	return nil
}

func (c *Comics) parseLastModifyTime() error {
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

func (c *Comics) WriteMetadata() error {
	metaPath := c.getMetadataPath()
	log.Debugf("metadata path:%v", metaPath)

	var buf bytes.Buffer
	buf.WriteString("标题: " + c.Title + "\n")
	buf.WriteString("链接: " + c.MainUrl + "\n")
	buf.WriteString("封面: " + c.CoverUrl + "\n")
	buf.WriteString("简介: " + c.Desc + "\n")
	buf.WriteString("更新时间: " + c.LastModifyTime + "\n")

	return c.writeFile(metaPath, buf.Bytes())
}

func (c *Comics) ParseMainPageUrls() error {
	var (
		existPages = make(map[string]bool, 8)
		pageUrls   = make([]string, 0, 8)
	)

	pageUrls = append(pageUrls, c.MainUrl)
	c.rootDoc.Find(".container .content-wrap .content .article-content .article-paging .post-page-numbers").Each(func(i int, s *goquery.Selection) {
		logField := log.Fields{"content": "get-main-page-url"}

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
	return c.writeContentMainFile()
}

func (c *Comics) writeContentMainFile() error {
	buf := bytes.Buffer{}

	for _, pageUrl := range c.PageUrls {
		pagePath, _ := c.getPageDataPath(pageUrl)
		_, pageName := filepath.Split(pageUrl)

		buf.WriteString(pagePath + " " + pageName + "\n")
	}

	contentPath := c.getContentDataPath("main")
	return c.writeFile(contentPath, buf.Bytes())
}

func (c *Comics) GetPageUrlsContent() error {
	logField := log.Fields{"content": "get-page-content"}

	for _, pageUrl := range c.PageUrls {
		logField["page-url"] = pageUrl

		htmlContent, err := c.getPageContent(pageUrl)
		if err != nil {
			log.WithFields(logField).WithField("position", "GetPageContent").Error(err)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
		if err != nil {
			log.WithFields(logField).WithField("position", "ParseHtmlContent").Error(err)
			continue
		}

		c.pageDocs[pageUrl] = doc
		log.WithFields(logField).Debug("parse page content success")
	}

	return nil
}

func (c *Comics) getPageContent(pageUrl string) (htmlContent []byte, err error) {
	pagePath, err := c.getPageDataPath(pageUrl)
	if err != nil {
		return
	}

	htmlContent, err = c.getPageContentInDisk(pagePath)
	if err == nil {
		return
	}

	htmlContent, err = DownloadPage(pageUrl, c.Debug)
	if err != nil {
		return
	}

	if err = c.writeFile(pagePath, htmlContent); err != nil {
		log.Errorf("write file:%v failed, err:%v", pagePath, err)
	}

	return
}

func (c *Comics) getPageContentInDisk(pagePath string) ([]byte, error) {
	stat, err := os.Stat(pagePath)
	if err != nil {
		return nil, err
	}

	if stat.Size() <= 0 {
		_ = os.Remove(pagePath)
		return nil, errors.New("empty file")
	}

	return os.ReadFile(pagePath)
}

func (c *Comics) writeFile(path string, data []byte) error {
	dir, file := filepath.Split(path)
	log.Debugf("dir:%v, file:%v", dir, file)

	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	if err := os.WriteFile(path, data, 0666); err != nil {
		return err
	}

	return nil
}

func (c *Comics) getMetadataPath() string {
	return filepath.Join(c.RootPath, c.EnTitle, DefaultMetadataPath, "base")
}

func (c *Comics) getContentDataPath(fname string) string {
	return filepath.Join(c.RootPath, c.EnTitle, DefaultContentDataPath, fname)
}

func (c *Comics) getPageDataPath(pageUrl string) (string, error) {
	logField := log.Fields{"content": "page-data-path", "page-url": pageUrl}

	pos := strings.LastIndex(pageUrl, "/")
	if pos == -1 {
		log.WithFields(logField).WithField("position", "FindLastSlash").Error("invalid page url")
		return "", errors.New("invalid page url")
	}

	return filepath.Join(c.RootPath, c.EnTitle, DefaultPageDataPath, pageUrl[pos+1:]), nil
}

func (c *Comics) getImageDataPath(imageUrl string) (string, error) {
	logField := log.Fields{"content": "image-data-path", "image-url": imageUrl}

	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseImageURIFailed").Error(err)
		return "", err
	}

	return filepath.Join(c.RootPath, c.EnTitle, DefaultImageDataPath, u.Path), nil
}

func (c *Comics) GetImageUrls() error {
	logField := log.Fields{"content": "get-image-url"}

	for pageUrl, doc := range c.pageDocs {
		logField["page-url"] = pageUrl

		imageUrls, err := c.getImageUrl(doc)
		if err != nil {
			return err
		}

		if err = c.writeContentPageFile(pageUrl, imageUrls); err != nil {
			return err
		}

		c.ImageUrls = append(c.ImageUrls, imageUrls...)
	}

	return nil
}

func (c *Comics) writeContentPageFile(pageUrl string, imageUrls []string) error {
	buf := bytes.Buffer{}

	for _, imageUrl := range imageUrls {
		imagePath, _ := c.getPageDataPath(imageUrl)
		buf.WriteString(imagePath + "\n")
	}

	_, file := filepath.Split(pageUrl)
	item := strings.Split(file, ".")

	return c.writeFile(c.getContentDataPath(item[0]), buf.Bytes())
}

func (c *Comics) getImageUrl(doc *goquery.Document) ([]string, error) {
	existImages := make(map[string]bool, 8)
	imageUrls := make([]string, 0, 8)

	doc.Find(".container .content-wrap .content .article-content p img").Each(func(i int, s *goquery.Selection) {
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
			log.WithFields(logField).WithField("position", "InvalidImageUrl").Info("invalid")
			return
		}

		imageUrls = append(imageUrls, src)
		existImages[src] = true
	})

	return imageUrls, nil
}

func (c *Comics) GetImagesContent() error {
	tasks := make(chan string, 1000)

	go func() {
		for _, imageUrl := range c.ImageUrls {
			tasks <- imageUrl
		}

		log.Debug("task send finish")
		close(tasks)
	}()

	for {
		time.Sleep(2 * time.Second)

		imageUrl, ok := <-tasks
		if !ok {
			log.Debug("task receive finish")
			break
		}

		log.Debugf("receive image, url:%v", imageUrl)

		if err := c.getImageContent(imageUrl); err != nil {
			log.Errorf("image url:%v, download failed, err:%v", imageUrl, err)
		}

		log.Debugf("download image:%v success", imageUrl)
	}

	return nil
}

func (c *Comics) getImageContent(imageUrl string) error {
	imagePath, err := c.getImageDataPath(imageUrl)
	if err != nil {
		return err
	}

	log.Debugf("image path:%v", imagePath)

	if err = c.isImageExist(imagePath); err != nil {
		return err
	}

	if err = c.downloadImageContent(imagePath, imageUrl); err != nil {
		return err
	}

	return nil
}

func (c *Comics) isImageExist(imagePath string) error {
	stat, err := os.Stat(imagePath)
	if err != nil {
		return err
	}

	if stat.Size() <= 0 {
		_ = os.Remove(imagePath)
		return errors.New("invalid image file")
	}

	return nil
}

func (c *Comics) downloadImageContent(imagePath, imageUrl string) error {
	return DownloadImage(imagePath, imageUrl)
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
