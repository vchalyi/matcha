package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mmcdole/gofeed"
	_ "modernc.org/sqlite"
)

func main() {
	cfg := bootstrapConfig()
	db, err := newDB(cfg.GetDbPath())
	if err != nil {
		fmt.Printf("Failed to open database: %v\n", err)
		return
	}
	defer db.Close()

	writer := getWriter(cfg)
	displayWeather(writer, cfg)
	displaySunriseSunset(writer, cfg)

	fp := gofeed.NewParser()
	generateAnalysis(db, fp, writer, cfg)

	feeds := cfg.GetAllFeeds()
	for i, feed := range feeds {
		fmt.Printf("[%d/%d] Feed: %s\n", i+1, len(feeds), feed.url)
		parsedFeed := parseFeed(fp, feed.url, feed.limit)

		if parsedFeed == nil {
			continue
		}

		fmt.Println("Items: ", len(parsedFeed.Items))

		var wg sync.WaitGroup
		results := make([]string, len(parsedFeed.Items))

		for j, item := range parsedFeed.Items {
			wg.Add(1)
			go func(idx int, item *gofeed.Item) {
				defer wg.Done()
				fmt.Printf("Item: %s\n", item.Title)
				itemStr := processFeedItem(db, writer, parsedFeed, feed, item, cfg)
				results[idx] = itemStr
			}(j, item)
		}

		wg.Wait()

		var feedStr string
		for _, itemStr := range results {
			feedStr += itemStr
		}

		if feedStr != "" {
			writeFeed(writer, parsedFeed, feedStr)
		}

		fmt.Println(strings.Repeat("*", 50))
	}

	fmt.Println("\nAll feeds processed.")
}
