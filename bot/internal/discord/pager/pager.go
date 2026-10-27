// Package pager splits long line lists into Discord messages with
// Previous/Next buttons. A view registers a Source under a name; its buttons
// carry "page:<name>:<page>", so the buttons handler routes every view's
// clicks through Handle without per-view code.
package pager

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

const buttonPrefix = "page:"

// discord msg limit is 2k, rest is room for the page footer
const pageLimit = 1800

// Source is one paginated view. Lines is called on every render, so pages
// reflect the current state when a button is clicked.
type Source struct {
	Header string
	Empty  string
	Lines  func(client *bot.Client, guildID snowflake.ID) []string
}

var sources = map[string]Source{}

// Register adds a view; call it from init. name must not contain ':'.
func Register(name string, s Source) {
	if strings.Contains(name, ":") {
		panic("pager: name contains ':': " + name)
	}
	if _, dup := sources[name]; dup {
		panic("pager: duplicate view " + name)
	}
	sources[name] = s
}

// Render returns page (0-based, clamped) of view name, with pager buttons
// when there is more than one page.
func Render(client *bot.Client, name string, guildID snowflake.ID, page int) (string, []discord.LayoutComponent) {
	src, ok := sources[name]
	if !ok {
		return "Unknown view.", nil
	}
	pages := split(src.Header, src.Lines(client, guildID))
	if len(pages) == 0 {
		return src.Empty, nil
	}
	if len(pages) == 1 {
		return src.Header + pages[0], nil
	}
	page = min(max(page, 0), len(pages)-1)
	content := fmt.Sprintf("%s%s\nPage %d/%d", src.Header, pages[page], page+1, len(pages))
	id := func(p int) string { return buttonPrefix + name + ":" + strconv.Itoa(p) }
	row := discord.NewActionRow(
		discord.NewSecondaryButton("Previous", id(page-1)).WithDisabled(page == 0),
		discord.NewSecondaryButton("Next", id(page+1)).WithDisabled(page == len(pages)-1),
	)
	return content, []discord.LayoutComponent{row}
}

// split packs lines into page bodies that fit pageLimit with header. A line
// too long for a page on its own is cut at a rune boundary.
func split(header string, lines []string) []string {
	room := max(pageLimit-len(header), 1)
	var pages []string
	b := &strings.Builder{}
	for _, l := range lines {
		l = truncate(l, room-1) + "\n"
		if b.Len() > 0 && b.Len()+len(l) > room {
			pages = append(pages, b.String())
			b.Reset()
		}
		b.WriteString(l)
	}
	if b.Len() > 0 {
		pages = append(pages, b.String())
	}
	return pages
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// Handle serves a pager button click, reporting false for other buttons.
func Handle(client *bot.Client, i *events.ComponentInteractionCreate) (bool, error) {
	rest, ok := strings.CutPrefix(i.Data.CustomID(), buttonPrefix)
	if !ok {
		return false, nil
	}
	name, page, ok := strings.Cut(rest, ":")
	p, err := strconv.Atoi(page)
	if !ok || err != nil || i.GuildID() == nil {
		return true, i.DeferUpdateMessage()
	}
	content, components := Render(client, name, *i.GuildID(), p)
	if components == nil {
		components = []discord.LayoutComponent{}
	}
	return true, i.UpdateMessage(discord.NewMessageUpdate().WithContent(content).WithComponents(components...))
}
