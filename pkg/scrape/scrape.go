package scrape

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	SupportImageSuffix = map[string]bool{
		"jpg":  true,
		"jpeg": true,
		"png":  true,
		"webp": true,
	}
)

const (
	DefaultTimeout       = 10 // 秒
	DefaultMetadataPath  = "./data/meta"
	DefaultImageDataPath = "./data/images"
)

type Comics struct {
	Debug          bool
	MetadataPath   string
	ImageDataPath  string
	Timeout        int
	Url            string
	Title          string
	EnTitle        string
	Desc           string
	LastModifyTime string
	CoverUrl       string
	PageUrls       []string
	ImageUrls      []string
	htmlContent    []byte
	rootDoc        *goquery.Document
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
	fmt.Fprintf(os.Stderr, "scrape config: %+v", c)

	return &Comics{
		Debug:     c.Debug,
		Url:       c.Url,
		PageUrls:  []string{},
		ImageUrls: []string{},
	}
}

func (c *Comics) Scrape() error {
	if err := c.GetMainContent(); err != nil {
		return err
	}

	if err := c.GetBasicInfo(); err != nil {
		return err
	}

	return nil
}

func (c *Comics) GetMainContent() error {
	var (
		err error
	)

	c.htmlContent, err = c.GetHtmlContent(c.Url)
	if err != nil {
		return err
	}

	c.rootDoc, err = goquery.NewDocumentFromReader(bytes.NewReader(c.htmlContent))
	if err != nil {
		return err
	}

	return nil
}

// GetBasicInfo Get Comic's title, cover, desc etc.
func (c *Comics) GetBasicInfo() error {
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
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-title").Each(func(i int, s *goquery.Selection) {
		c.Title = s.Text()
		c.EnTitle = ParseCnToEn(c.Title)
	})

	return nil
}

func (c *Comics) GetDesc() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .dis").Each(func(i int, s *goquery.Selection) {
		c.Desc = s.Text()
	})

	return nil
}

func (c *Comics) GetCoverUrl() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .c-img img").Each(func(i int, s *goquery.Selection) {
		src, exist := s.Attr("src")
		if !exist {
			fmt.Printf("no src attr\n")
			return
		}

		fmt.Printf("index:%v, src:%v\n", i, src)
		c.Url = src
	})

	return nil
}

func (c *Comics) GetLastModifyTime() error {
	c.rootDoc.Find(".container .content-wrap .content .article-header .article-meta li").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		fmt.Printf("last modify time:%v\n", text)

		items := strings.Split(text, ":")
		if len(items) != 2 {
			return
		}

		if _, err := time.ParseInLocation("2006-01-02", items[1], time.Local); err != nil {
			return
		}

		c.LastModifyTime = items[1]
	})

	return nil
}

func (c *Comics) GetPageUrl(mainUrl string) ([]string, error) {
	existPages := make(map[string]bool, 8)
	pageUrls := make([]string, 0, 8)
	pageUrls = append(pageUrls, mainUrl)

	c.rootDoc.Find(".container .content-wrap .content .article-content .article-paging .post-page-numbers").Each(func(i int, s *goquery.Selection) {
		href, exist := s.Attr("href")
		if !exist {
			fmt.Printf("no href attr\n")
			return
		}

		fmt.Printf("index:%v, href:%v\n", i, href)

		if existPages[href] {
			fmt.Printf("href:%v, already processed\n", href)
			return
		}

		if !IsValidPageUrl(href) {
			return
		}

		pageUrls = append(pageUrls, href)
		existPages[href] = true
	})

	// fmt.Printf("page urls:%+v\n", pageUrls)
	c.PageUrls = pageUrls
	return pageUrls, nil
}

func (c *Comics) GetImageUrl(pageUrl string) ([]string, error) {
	existImages := make(map[string]bool, 8)
	imageUrls := make([]string, 0, 8)

	c.rootDoc.Find(".container .content-wrap .content .article-content p img").Each(func(i int, s *goquery.Selection) {
		src, exist := s.Attr("src")
		if !exist {
			fmt.Printf("no src attr\n")
			return
		}

		fmt.Printf("index:%v, src:%v\n", i, src)

		if existImages[src] {
			fmt.Printf("href:%v, already processed\n", src)
			return
		}

		if !IsValidImageUrl(src) {
			return
		}

		imageUrls = append(imageUrls, src)
		existImages[src] = true
	})

	c.ImageUrls = imageUrls
	return imageUrls, nil
}

func (c *Comics) GetHtmlContent(url string) ([]byte, error) {
	if c.Debug {
		return badMan, nil
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Host", "www.san499.com")
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/112.0")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Referer", "https://www.san499.com/")
	req.Header.Add("Sec-Fetch-Mode", "no-cors")
	req.Header.Add("Sec-Fetch-Site", "cross-site")
	req.Header.Add("TE", "trailers")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func IsValidPageUrl(pageUrl string) bool {
	u, err := url.ParseRequestURI(pageUrl)
	if err != nil {
		fmt.Printf("parse url:%v failed, err:%v\n", pageUrl, err)
		return false
	}

	fmt.Printf("url:%v, path:%v\n", pageUrl, u.Path)
	dir, file := filepath.Split(u.Path)
	fmt.Printf("dir:%v, file:%v\n", dir, file)

	file = strings.ToLower(file)
	if !strings.HasPrefix(file, "page") {
		fmt.Printf("invalid page url, no page prefix\n")
		return false
	}

	if !strings.HasSuffix(file, ".html") {
		fmt.Printf("invalid page url, no html suffix\n")
		return false
	}

	return true
}

func IsValidImageUrl(imageUrl string) bool {
	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		fmt.Printf("parse url:%v failed, err:%v\n", imageUrl, err)
		return false
	}

	fmt.Printf("url:%v, path:%v\n", imageUrl, u.Path)
	dir, file := filepath.Split(u.Path)
	fmt.Printf("dir:%v, file:%v\n", dir, file)

	file = strings.ToLower(file)
	pos := strings.LastIndex(file, ".")
	if pos == -1 {
		fmt.Printf("image url:%v, invalid suffix\n", imageUrl)
		return false
	}

	suffix := file[pos+1:]
	if !SupportImageSuffix[suffix] {
		fmt.Printf("invalid image url, not support suffix:[%v]\n", suffix)
		return false
	}

	return true
}

func DownloadImage(imageUrl string) error {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest("GET", imageUrl, nil)
	if err != nil {
		return err
	}

	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		fmt.Printf("parse url:%v failed, err:%v\n", imageUrl, err)
		return err
	}

	req.Header.Add("Host", u.Host)
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/112.0")
	req.Header.Add("Accept", "image/avif,image/webp,*/*")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Referer", "https://www.san499.com/")
	req.Header.Add("Sec-Fetch-Dest", "image")
	req.Header.Add("Sec-Fetch-Mode", "no-cors")
	req.Header.Add("Sec-Fetch-Site", "cross-site")
	req.Header.Add("TE", "trailers")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("response size:", len(body))

	dir, file := filepath.Split(u.Path)
	fmt.Printf("dir:%v, file:%v\n", dir, file)

	if err = os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	if err = os.WriteFile(u.Path, body, 0666); err != nil {
		return err
	}

	return nil
}
