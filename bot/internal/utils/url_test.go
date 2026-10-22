package utils

import "testing"

func TestClassifyURL(t *testing.T) {
	cases := map[string]string{
		"https://cdn.discordapp.com/attachments/1/2/cat.PNG?ex=abc": "image",
		"https://i.imgur.com/abc.gifv":                              "gif",
		"https://media.tenor.com/xyz/view":                          "gif",
		"https://m.youtube.com/watch?v=1":                           "video",
		"https://clips.twitch.tv/Some":                              "video",
		"https://example.com/clip.webm":                             "video",
		"https://inbox.com/page":                                    "",
		"https://example.com/":                                      "",
	}
	for u, want := range cases {
		if got := ClassifyURL(u); got != want {
			t.Errorf("ClassifyURL(%q) = %q, want %q", u, got, want)
		}
	}
}
