package scrape

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

func DownloadPage(url string, debug bool) ([]byte, error) {
	if debug {
		return badMan, nil
	}

	logField := log.Fields{"content": "html-content", "url": url}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithFields(logField).WithField("position", "new http request failed").Error(err)
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
		log.WithFields(logField).WithField("position", "DoHttpRequestFailed").Error(err)
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(logField).WithField("position", "ReadBodyFailed").Error(err)
		return nil, err
	}

	log.WithFields(logField).Debug("success")
	return body, nil
}

func DownloadImage(imageUrl string, path string) error {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	logField := log.Fields{"content": "download-image", "image-url": imageUrl}

	req, err := http.NewRequest("GET", imageUrl, nil)
	if err != nil {
		log.WithFields(logField).WithField("position", "NewHttpRequestFailed").Error(err)
		return err
	}

	u, err := url.ParseRequestURI(imageUrl)
	if err != nil {
		log.WithFields(logField).WithField("position", "ParseImageURIFailed").Error(err)
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
		log.WithFields(logField).WithField("position", "DoHttpRequestFailed").Error(err)
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(logField).WithField("position", "ReadBodyFailed").Error(err)
		return err
	}

	log.WithFields(logField).Debugf("response size:%v", len(body))

	dir, file := filepath.Split(u.Path)
	log.WithFields(logField).Debugf("dir:%v, file:%v", dir, file)

	if err = os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		log.WithFields(logField).WithField("position", "MkDirFailed").Error(err)
		return err
	}

	if err = os.WriteFile(u.Path, body, 0666); err != nil {
		log.WithFields(logField).WithField("position", "WriteImageToFileFailed").Error(err)
		return err
	}

	log.WithFields(logField).Debug("write image to file success")
	return nil
}
