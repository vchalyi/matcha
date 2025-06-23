package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mmcdole/gofeed"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

const defaultLimit = 20 // default number of articles per feed for analysis
var model = openai.GPT4o

func generateAnalysis(db *sql.DB, fp *gofeed.Parser, writer Writer, cfg Config) {
	if !viper.IsSet("analyst_feeds") || !viper.IsSet("analyst_prompt") {
		return
	}

	analystFeeds := viper.GetStringSlice("analyst_feeds")
	analystPrompt := viper.GetString("analyst_prompt")
	analystModel := viper.GetString("analyst_model")

	var articleTitles []string
	for _, feedURL := range analystFeeds {
		parsedFeed := parseFeed(fp, feedURL, defaultLimit)
		if parsedFeed == nil {
			continue
		}
		for _, item := range parsedFeed.Items {
			seen, seen_today, summary := isSeenArticle(db, item, "#analyst")
			if seen {
				continue
			}
			articleTitles = append(articleTitles, item.Title+":  "+item.Description) // add also description for better context
			if !seen_today {
				addToSeenTable(db, item.Link+"#analyst", summary)
			}
		}
	}

	if len(articleTitles) == 0 {
		return
	}

	prompt := fmt.Sprintf("%s\n\n%s", analystPrompt, strings.Join(articleTitles, "\n"))
	analysis := getLLMAnalysis(prompt, analystModel, cfg)

	if analysis != "" {
		writer.write("\n## Daily Analysis:\n")
		writer.write(analysis + "\n")
	}
}

func getLLMAnalysis(prompt string, analystModel string, cfg Config) string {
	clientConfig := openai.DefaultConfig(cfg.OpenaiApiKey)
	if cfg.OpenaiBaseURL != "" {
		clientConfig.BaseURL = cfg.OpenaiBaseURL
	}
	if analystModel != "" {
		model = analystModel
	}
	client := openai.NewClientWithConfig(clientConfig)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return ""
	}

	return resp.Choices[0].Message.Content
}
