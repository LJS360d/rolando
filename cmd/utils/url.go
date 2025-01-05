package utils

import (
	"net/url"
	"strings"
)

func ExtractUrlInfo(inputUrl string) (domain string, extension string) {
	parsedUrl, err := url.Parse(inputUrl)
	if err != nil {
		return "", ""
	}
	domain = parsedUrl.Hostname()
	extension = parsedUrl.Path[strings.LastIndex(parsedUrl.Path, ".")+1:]
	return
}

func IsGif(url string) bool {
	supportedExtensions := []string{"gif"}
	supportedDomains := []string{"tenor.com", "giphy.com"}
	return isSupportedUrl(strings.TrimSpace(url), supportedExtensions, supportedDomains)
}

func IsImage(url string) bool {
	supportedExtensions := []string{"png", "jpg", "jpeg", "webp"}
	supportedDomains := []string{"imgur.com"}
	return isSupportedUrl(strings.TrimSpace(url), supportedExtensions, supportedDomains)
}

func IsVideo(url string) bool {
	supportedExtensions := []string{"mp4", "mov"}
	supportedDomains := []string{"www.youtube.com", "youtube.com", "youtu.be"}
	return isSupportedUrl(strings.TrimSpace(url), supportedExtensions, supportedDomains)
}

func isSupportedUrl(url string, extensions []string, domains []string) bool {
	domain, extension := ExtractUrlInfo(url)

	for _, ext := range extensions {
		if extension == ext {
			return true
		}
	}

	for _, d := range domains {
		if domain == d {
			return true
		}
	}

	return false
}
