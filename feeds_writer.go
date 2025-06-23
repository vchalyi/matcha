package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"
	"github.com/mmcdole/gofeed"
)

var mdPrefix, mdSuffix string
var currentDate = time.Now().Format("2006-01-02")

type RSS struct {
	url       string
	limit     int
	summarize bool
}

type Writer interface {
	write(body string)
	writeLink(title string, url string, newline bool, readingTime string) string
	writeSummary(content string, newline bool) string
	writeFavicon(s *gofeed.Feed) string
}

func getWriter(cfg Config) Writer {
	if cfg.TerminalMode {
		return TerminalWriter{}
	}
	return MarkdownWriter{MarkdownDirPath: cfg.MarkdownDirPath}
}

func fatal(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func getReadingTime(link string) string {
	article, err := readability.FromURL(link, 30*time.Second)
	if err != nil {
		return "" // Just dont display any reading time if can't get the article text
	}

	// get number of words in a string
	words := strings.Fields(article.TextContent)

	// assuming average reading time is 200 words per minute calculate reading time of the article
	readingTime := float64(len(words)) / float64(200)
	minutes := int(readingTime)

	// if minutes is zero return an empty string
	if minutes == 0 {
		return ""
	}

	return strconv.Itoa(minutes) + " min"
}

func (w MarkdownWriter) writeFavicon(s *gofeed.Feed) string {
	var src string
	if s.FeedLink == "" {
		// default feed favicon
		return "🍵"

	} else {
		u, err := url.Parse(s.FeedLink)
		if err != nil {
			fmt.Println(err)
		}
		src = "https://www.google.com/s2/favicons?sz=32&domain=" + u.Hostname()
	}
	// if s.Title contains "hacker news"
	if strings.Contains(s.Title, "Hacker News") {
		src = "https://news.ycombinator.com/favicon.ico"
	}

	//return html image tag of favicon
	return fmt.Sprintf("<img src=\"%s\" width=\"32\" height=\"32\" />", src)
}

func ExtractImageTagFromHTML(htmlText string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return "" // Error occurred while parsing HTML
	}

	imgTags := doc.Find("img")

	if imgTags.Length() == 0 {
		return "" // No img tag found, return empty string
	}

	firstImgTag := imgTags.First()

	width := firstImgTag.AttrOr("width", "")
	height := firstImgTag.AttrOr("height", "")

	// If both width and height are present, calculate the aspect ratio and set the maximum width
	if width != "" && height != "" {
		widthInt, _ := strconv.Atoi(width)
		heightInt, _ := strconv.Atoi(height)

		if widthInt > 0 && heightInt > 0 {
			aspectRatio := float64(widthInt) / float64(heightInt)
			maxWidth := 400

			if widthInt > maxWidth {
				widthInt = maxWidth
				heightInt = int(float64(widthInt) / aspectRatio)
			}

			firstImgTag.SetAttr("width", fmt.Sprintf("%d", widthInt))
			firstImgTag.SetAttr("height", fmt.Sprintf("%d", heightInt))
		}
	}

	html, err := goquery.OuterHtml(firstImgTag)
	if err != nil {
		return "" // Error occurred while extracting the HTML of the img tag
	}

	return html // Return the modified img tag
}

// Parses the feed URL and returns the feed object
func parseFeed(fp *gofeed.Parser, url string, limit int) *gofeed.Feed {
	feed, err := fp.ParseURL(url)
	if err != nil {
		fmt.Printf("Error parsing %s with error: %s", url, err)
		return nil
	}

	if len(feed.Items) > limit {
		feed.Items = feed.Items[:limit]
	}

	return feed
}

// Processes a single feed item and returns its string representation
func processFeedItem(db *sql.DB, w Writer, feed *gofeed.Feed, rss RSS, item *gofeed.Item, cfg Config) string {
	seen, seen_today, summary := isSeenArticle(db, item, "")
	if seen {
		return ""
	}
	title, link := item.Title, item.Link
	if summary == "" {
		summary = getSummary(cfg, rss, item, link)
	}
	var itemStr string

	// Add the comments link if it's a Hacker News feed
	if strings.Contains(feed.Link, "news.ycombinator.com") {
		commentsLink, commentsCount := getCommentsInfo(item)
		if commentsCount < 100 {
			itemStr += w.writeLink("💬 ", commentsLink, false, "")
		} else {
			itemStr += w.writeLink("🔥 ", commentsLink, false, "")
		}
	}

	// Add the Instapaper link if enabled
	if cfg.Instapaper && !cfg.TerminalMode {
		itemStr += getInstapaperLink(item.Link)
	}

	// Support RSS with no Title (such as Mastodon), use Description instead
	if title == "" {
		title = stripHtmlRegex(item.Description)
	}

	timeInMin := ""
	if cfg.ReadingTime {
		timeInMin = getReadingTime(link)
	}

	itemStr += w.writeLink(title, link, true, timeInMin)
	if rss.summarize {
		itemStr += w.writeSummary(summary, true)
	}

	if cfg.ShowImages && !cfg.TerminalMode {
		img := ExtractImageTagFromHTML(item.Content)
		if img != "" {
			itemStr += img + "\n"
		}
	}

	// Add the item to the seen table if not seen today
	if !seen_today {
		addToSeenTable(db, item.Link, summary)
	}

	return itemStr
}

// Writes the feed and its items
func writeFeed(w Writer, feed *gofeed.Feed, items string) {
	w.write(fmt.Sprintf("\n### %s  %s\n%s", w.writeFavicon(feed), feed.Title, items))
}

// Returns the summary for the given feed item
func getSummary(cfg Config, rss RSS, item *gofeed.Item, link string) string {
	if !rss.summarize {
		return ""
	}

	summary := getSummaryFromLink(cfg, link)
	if summary == "" {
		summary = item.Description
	}

	return summary
}

// Returns the comments link and count for the given feed item
func getCommentsInfo(item *gofeed.Item) (string, int) {
	first_index := strings.Index(item.Description, "Comments URL") + 23
	comments_url := item.Description[first_index : first_index+45]
	// Find Comments number
	first_comments_index := strings.Index(item.Description, "Comments:") + 10
	// replace </p> with empty string
	comments_number := strings.Replace(item.Description[first_comments_index:], "</p>\n", "", -1)
	comments_number_int, _ := strconv.Atoi(comments_number)
	// return the link and the number of comments
	return comments_url, comments_number_int
}

func addToSeenTable(db *sql.DB, link string, summary string) {
	stmt, err := db.Prepare("INSERT INTO seen(url, date, summary) values(?,?,?)")
	fatal(err)
	res, err := stmt.Exec(link, currentDate, summary)
	fatal(err)
	_ = res
	stmt.Close()
}

func getInstapaperLink(link string) string {
	return "[<img height=\"16\" src=\"https://staticinstapaper.s3.dualstack.us-west-2.amazonaws.com/img/favicon.png\">](https://www.instapaper.com/hello2?url=" + link + ")"
}

func isSeenArticle(db *sql.DB, item *gofeed.Item, postfix string) (seen bool, today bool, summaryText string) {
	var url string
	var date string
	var summary sql.NullString
	err := db.QueryRow("SELECT url, date, summary FROM seen WHERE url=?", item.Link+postfix).Scan(&url, &date, &summary)
	if err != nil && err != sql.ErrNoRows {
		fmt.Println(err)
		return false, false, ""
	}

	if summary.Valid {
		summaryText = summary.String
	} else {
		summaryText = ""
	}

	seen = url != "" && date != currentDate
	today = url != "" && date == currentDate
	return seen, today, summaryText
}
