package utils

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	ReURL = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s]+`)
)

func ExtractUrls(text string) []string {
	return ReURL.FindAllString(text, -1)
}

var mediaExtensions = map[string]string{
	"gif":  "gif",
	"gifv": "gif",
	"png":  "image",
	"jpg":  "image",
	"jpeg": "image",
	"jfif": "image",
	"webp": "image",
	"avif": "image",
	"bmp":  "image",
	"mp4":  "video",
	"mov":  "video",
	"webm": "video",
	"m4v":  "video",
}

// mediaDomains are hosts whose links Discord embeds from page metadata
// (OpenGraph/oEmbed), so the URL itself is an HTML page and cannot be sniffed.
// A host matches itself and its subdomains.
var mediaDomains = map[string]string{
	"tenor.com":        "gif",
	"giphy.com":        "gif",
	"redgifs.com":      "gif",
	"imgur.com":        "image",
	"pinterest.com":    "image",
	"pin.it":           "image",
	"pixiv.net":        "image",
	"pximg.net":        "image",
	"flickr.com":       "image",
	"staticflickr.com": "image",
	"twitter.com":      "image",
	"x.com":            "image",
	"fixvx.com":        "image",
	"fxtwitter.com":    "image",
	"vxtwitter.com":    "image",
	"fixupx.com":       "image",
	"bsky.app":         "image",
	"youtube.com":      "video",
	"youtu.be":         "video",
	"streamable.com":   "video",
	"tiktok.com":       "video",
	"vimeo.com":        "video",
	"clips.twitch.tv":  "video",
	"medal.tv":         "video",
}

// ClassifyURL returns the media kind of url ("gif", "image", "video") from its
// extension or host, or "" when that is not enough to tell.
func ClassifyURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if k, ok := mediaExtensions[strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))]; ok {
		return k
	}
	for host := strings.ToLower(u.Hostname()); host != ""; {
		if k, ok := mediaDomains[host]; ok {
			return k
		}
		_, host, _ = strings.Cut(host, ".")
	}
	return ""
}
