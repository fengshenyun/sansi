package scrape

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
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
		log.Error(err)
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

	if err := c.GetImagesContent(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) Validity() error {
	if c.MainUrl == "" {
		return errors.New("empty main url")
	}

	u, err := url.ParseRequestURI(c.MainUrl)
	if err != nil {
		return errors.Wrap(err, "parse main url failed")
	}

	dir, file := filepath.Split(u.Path)
	log.Debugf("dir:%v, file:%v", dir, file)

	if !strings.HasSuffix(file, ".html") {
		return errors.New("no html suffix")
	}

	pos := strings.LastIndex(file, ".html")
	num, err := strconv.Atoi(file[:pos])
	if err != nil {
		return errors.Wrap(err, "invalid comic number")
	}

	c.Number = num
	log.Debugf("mainPage:%v, verified success", c.MainUrl)
	return nil
}

func (c *Comics) GetMainContent() error {
	var (
		err error
	)

	c.rootHtmlContent, err = DownloadPage(c.MainUrl, c.Debug)
	if err != nil {
		return errors.Wrap(err, "download main page failed")
	}

	c.rootDoc, err = goquery.NewDocumentFromReader(bytes.NewReader(c.rootHtmlContent))
	if err != nil {
		return errors.Wrap(err, "parse main page html failed")
	}

	log.Debug("download main page success")
	return nil
}

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
	})

	if c.Title == "" {
		return errors.New("no title")
	}

	log.Debugf("title:%v, enTitle:%v", c.Title, c.EnTitle)
	return nil
}

func (c *Comics) parseDesc() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .dis").Each(func(i int, s *goquery.Selection) {
		c.Desc = s.Text()
		c.Desc = strings.Trim(c.Desc, " \n\t\r")
	})

	log.Debugf("desc:%v", c.Desc)
	return nil
}

func (c *Comics) parseCoverUrl() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .c-img img").Each(func(i int, s *goquery.Selection) {
		src, exist := s.Attr("src")
		if !exist {
			log.Info("no src attr")
			return
		}

		c.CoverUrl = src
	})

	if c.CoverUrl == "" {
		return errors.New("no cover url")
	}

	log.Debugf("cover url:%v", c.CoverUrl)
	return nil
}

func (c *Comics) parseLastModifyTime() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-meta li").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		text = strings.Trim(text, " \n\t\r")

		items := strings.Split(text, ":")
		if len(items) != 2 {
			log.Info("invalid format")
			return
		}

		if _, err := time.ParseInLocation("2006-01-02", items[1], time.Local); err != nil {
			log.Info("no include yyyy-mm-dd time")
			return
		}

		c.LastModifyTime = items[1]
	})

	log.Debugf("lastModifyTime:%v", c.LastModifyTime)
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

	if err := c.writeFile(metaPath, buf.Bytes()); err != nil {
		log.Errorf("write metadata failed, err:%v", err)
		return err
	}

	log.Debugf("write metadata success.")
	return nil
}

func (c *Comics) ParseMainPageUrls() error {
	var (
		existPages = make(map[string]bool, 8)
		pageUrls   = make([]string, 0, 8)
	)

	pageUrls = append(pageUrls, c.MainUrl)
	c.rootDoc.Find(".container .content-wrap .content .article-content .article-paging .post-page-numbers").Each(func(i int, s *goquery.Selection) {
		href, exist := s.Attr("href")
		if !exist {
			log.Info("no href attr")
			return
		}

		log.Infof("i:%v,href:%v", i, href)

		if existPages[href] {
			log.Infof("href:%v, already process", href)
			return
		}

		if !c.IsValidPageUrl(href) {
			log.Infof("href:%v, invalid page url", href)
			return
		}

		pageUrls = append(pageUrls, href)
		existPages[href] = true
		log.Debugf("sub page url:%v", href)
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
	if err := c.writeFile(contentPath, buf.Bytes()); err != nil {
		log.Errorf("write data/main file failed, err:%v", err)
		return err
	}

	log.Errorf("write data/main file success")
	return nil
}

func (c *Comics) GetPageUrlsContent() error {
	for _, pageUrl := range c.PageUrls {
		htmlContent, err := c.getPageContent(pageUrl)
		if err != nil {
			log.Errorf("get pageUrl:%v content failed, err:%v", pageUrl, err)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
		if err != nil {
			log.Errorf("parse pageUrl:%v content failed, err:%v", pageUrl, err)
			continue
		}

		c.pageDocs[pageUrl] = doc
		log.Debug("pageUrl:%v, parse page content success", pageUrl)
	}

	return nil
}

func (c *Comics) getPageContent(pageUrl string) (htmlContent []byte, err error) {
	pagePath, err := c.getPageDataPath(pageUrl)
	if err != nil {
		log.Errorf("pageUrl:%v, get page path failed, err:%v", pageUrl, err)
		return
	}

	htmlContent, err = c.getPageContentInDisk(pagePath)
	if err == nil {
		log.Debugf("pageUrl:%v already in disk, no need download", pageUrl)
		return
	}

	htmlContent, err = DownloadPage(pageUrl, c.Debug)
	if err != nil {
		log.Errorf("pageUrl:%v, download failed, err:%v", pageUrl, err)
		return
	}

	if err = c.writeFile(pagePath, htmlContent); err != nil {
		log.Errorf("write file:%v failed, err:%v", pagePath, err)
	}

	log.Debugf("pageUrl:%v, download success", pageUrl)
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
	pos := strings.LastIndex(pageUrl, "/")
	if pos == -1 {
		log.Errorf("invalid pageUrl:%v", pageUrl)
		return "", errors.New("invalid page url")
	}

	if pageUrl == c.MainUrl {
		return filepath.Join(c.RootPath, c.EnTitle, DefaultPageDataPath, pageUrl[pos+1:]), nil
	}

	return filepath.Join(c.RootPath, c.EnTitle, DefaultPageDataPath, pageUrl[pos+1:]), nil
}

func (c *Comics) getImageDataPath(imageUrl string) (string, error) {
	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		log.Errorf("imageUrl:%v, parse failed, err:%v", imageUrl, err)
		return "", err
	}

	return filepath.Join(c.RootPath, c.EnTitle, DefaultImageDataPath, u.Path), nil
}

func (c *Comics) GetImageUrls() error {
	for pageUrl, doc := range c.pageDocs {
		imageUrls, err := c.getImageUrl(doc)
		if err != nil {
			log.Errorf("pageUrl:%v, get image urls from page url failed, err:%v", pageUrl, err)
			continue
		}

		log.Debugf("pageUrl:%v, imageUrls:%+v", pageUrl, imageUrls)

		if err = c.writeContentPageFile(pageUrl, imageUrls); err != nil {
			log.Errorf("pageUrl:%v, write image urls to file failed, err:%v", pageUrl, err)
			continue
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
		src, exist := s.Attr("src")
		if !exist {
			log.Info("no src attr")
			return
		}

		log.Infof("index:%v, src:%v", i, src)

		if existImages[src] {
			log.Infof("imageUrl:%v, already process", src)
			return
		}

		if !c.IsValidImageUrl(src) {
			log.Infof("imageUrl:%v, invalid image url", src)
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
			continue
		}

		log.Debugf("download image:%v success", imageUrl)
	}

	return nil
}

func (c *Comics) getImageContent(imageUrl string) error {
	imagePath, err := c.getImageDataPath(imageUrl)
	if err != nil {
		return errors.Wrapf(err, "get image path failed")
	}

	log.Debugf("imageUrl:%v, imagePath:%v", imageUrl, imagePath)

	if ok := c.isImageExist(imagePath); ok {
		log.Infof("imageUrl:%v already exist, no need download", imageUrl)
		return nil
	}

	if err = c.downloadImageContent(imagePath, imageUrl); err != nil {
		return errors.Wrap(err, "download image content failed")
	}

	log.Debugf("imageUrl:%v download content success", imageUrl)
	return nil
}

func (c *Comics) isImageExist(imagePath string) bool {
	stat, err := os.Stat(imagePath)
	if err != nil {
		return false
	}

	if stat.Size() <= 0 {
		_ = os.Remove(imagePath)
		return false
	}

	return true
}

func (c *Comics) downloadImageContent(imagePath, imageUrl string) error {
	return DownloadImage(imagePath, imageUrl)
}

func (c *Comics) IsValidPageUrl(pageUrl string) bool {
	var (
		pagePrefix = "page"
		pageSuffix = ".html"
	)

	if pageUrl == c.MainUrl {
		return true
	}

	u, err := url.ParseRequestURI(pageUrl)
	if err != nil {
		log.Errorf("pageUrl:%v parse failed, err:%v", pageUrl, err)
		return false
	}

	dir, file := filepath.Split(u.Path)
	log.Debugf("pageUrl:%v, dir:%v, file:%v", pageUrl, dir, file)

	file = strings.ToLower(file)
	if !strings.HasPrefix(file, pagePrefix) {
		log.Errorf("pageUrl:%v, no %v prefix", pageUrl, pagePrefix)
		return false
	}

	if !strings.HasSuffix(file, pageSuffix) {
		log.Errorf("pageUrl:%v, no %v suffix", pageUrl, pageSuffix)
		return false
	}

	log.Debugf("pageUrl:%v verify success", pageUrl)
	return true
}

func (c *Comics) IsValidImageUrl(imageUrl string) bool {
	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		log.Errorf("imageUrl:%v parse failed, err:%v", imageUrl, err)
		return false
	}

	dir, file := filepath.Split(u.Path)
	log.Debugf("imageUrl:%v, dir:%v, file:%v", imageUrl, dir, file)

	file = strings.ToLower(file)
	pos := strings.LastIndex(file, ".")
	if pos == -1 {
		log.Error("invalid suffix")
		return false
	}

	suffix := file[pos+1:]
	if !SupportImageSuffix[suffix] {
		log.Errorf("imageUrl:%v, unsupported image suffix:%v", imageUrl, suffix)
		return false
	}

	log.Debugf("imageUrl:%v verify success", imageUrl)
	return true
}
