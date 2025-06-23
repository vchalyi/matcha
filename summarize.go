package main

import (
	"context"
	"fmt"
	"time"

	readability "github.com/go-shiori/go-readability"
	openai "github.com/sashabaranov/go-openai"
)

func getSummaryFromLink(cfg Config, url string) string {
	article, err := readability.FromURL(url, 30*time.Second)
	if err != nil {
		fmt.Printf("Failed to parse %s, %v\n", url, err)
	}

	return summarize(article.TextContent, cfg)

}

func summarize(text string, cfg Config) string {
	if len(text) > cfg.SummaryArticleLengthLimit {
		text = text[:cfg.SummaryArticleLengthLimit]
	}

	// Dont summarize if the article is too short
	if len(text) < 200 {
		return ""
	}
	clientConfig := openai.DefaultConfig(cfg.OpenaiApiKey)
	if cfg.OpenaiBaseURL != "" {
		clientConfig.BaseURL = cfg.OpenaiBaseURL
	}
	model := openai.GPT3Dot5Turbo
	if cfg.OpenaiModel != "" {
		model = cfg.OpenaiModel
	}
	client := openai.NewClientWithConfig(clientConfig)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Summarize the following text:",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
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
