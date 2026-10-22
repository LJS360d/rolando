package jackbox

import "net/http"

const jackboxUserAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36`

func applyJackboxHTTPHeaders(req *http.Request) {
	req.Header.Set("User-Agent", jackboxUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://jackbox.tv")
	req.Header.Set("Referer", "https://jackbox.tv/")
}
