package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var config string = `markdown_dir_path:
feeds:
  - http://hnrss.org/best 10
  - https://waitbutwhy.com/feed
  - http://tonsky.me/blog/atom.xml
  - http://www.joelonsoftware.com/rss.xml
  - https://www.youtube.com/feeds/videos.xml?channel_id=UCHnyfMqiRRG1u-2MsSQLbXA
google_news_keywords: George Hotz,ChatGPT,Copenhagen
instapaper: true
weather_latitude: 37.77
weather_longitude: 122.41
terminal_mode: false
opml_file_path:
markdown_file_prefix:
markdown_file_suffix:
reading_time: false
sunrise_sunset: false
openai_api_key:
openai_base_url:
openai_model:
summary_feeds:
summary_article_lenght_limit: 5000
show_images: false
analyst_feeds:
  - https://feeds.bbci.co.uk/news/business/rss.xml
analyst_prompt:
analyst_model:
`

type Config struct {
	MarkdownDirPath           string
	Feeds                     []RSS
	SummaryFeeds              []RSS
	SummaryArticleLengthLimit int
	GoogleNewsKeywords        string
	GoogleNewsFeeds           []RSS
	Instapaper                bool
	WeatherLatitude           float64
	WeatherLongitude          float64
	TerminalMode              bool
	OpmlFilePath              string
	OpmlFeeds                 []RSS
	MarkdownFilePrefix        string
	MarkdownFileSuffix        string
	ReadingTime               bool
	SunriseSunset             bool
	OpenaiApiKey              string
	OpenaiBaseURL             string
	OpenaiModel               string
	ShowImages                bool
	DatabaseFilePath          string
}

func (cfg *Config) GetAllFeeds() []RSS {

	allFeeds := []RSS{}
	allFeeds = append(allFeeds, cfg.Feeds...)
	allFeeds = append(allFeeds, cfg.SummaryFeeds...)
	allFeeds = append(allFeeds, cfg.GoogleNewsFeeds...)
	allFeeds = append(allFeeds, cfg.OpmlFeeds...)

	return allFeeds
}

func (cfg *Config) GetDbPath() string {
	if cfg.DatabaseFilePath != "" {
		return cfg.DatabaseFilePath
	}

	databaseDirPath, err := os.UserConfigDir()
	fatal(err)
	databaseFilePath := filepath.Join(databaseDirPath, "brew", "matcha.db")
	fatal(os.MkdirAll(filepath.Dir(databaseFilePath), os.ModePerm))

	return databaseFilePath
}

func parseOPML(xmlContent []byte) []RSS {
	o := Opml{}
	OpmlSlice := []RSS{}
	decoder := xml.NewDecoder(strings.NewReader(string(xmlContent)))
	decoder.Strict = false
	if err := decoder.Decode(&o); err != nil {
		log.Println(err)
	}
	for _, outline := range o.Body.Outline {
		if outline.XmlUrl != "" {
			OpmlSlice = append(OpmlSlice, RSS{url: outline.XmlUrl, limit: 20})
		}
		for _, feed := range outline.Outline {
			if feed.XmlUrl != "" {
				OpmlSlice = append(OpmlSlice, RSS{url: feed.XmlUrl, limit: 20})
			}
		}
	}
	return OpmlSlice
}

func getFeedAndLimit(feedURL string) (string, int) {
	var limit = 20 // default limit
	chopped := strings.Split(feedURL, " ")
	if len(chopped) > 1 {
		var err error
		limit, err = strconv.Atoi(chopped[1])
		if err != nil {
			fatal(err)
		}
	}
	return chopped[0], limit
}

func bootstrapConfig() Config {
	currentDir, direrr := os.Getwd()
	if direrr != nil {
		log.Println(direrr)
	}

	cfg := Config{
		Feeds:           []RSS{},
		SummaryFeeds:    []RSS{},
		GoogleNewsFeeds: []RSS{},
		OpmlFeeds:       []RSS{},
		TerminalMode:    false,
	}

	// if -t parameter is passed overwrite terminal_mode setting in config.yml
	flag.BoolVar(&cfg.TerminalMode, "t", cfg.TerminalMode, "Run Matcha in Terminal Mode, no markdown files will be created")
	configFile := flag.String("c", "", "Config file path (if you want to override the current directory config.yaml)")
	opmlFile := flag.String("o", "", "OPML file path to append feeds from opml files")
	build := flag.Bool("build", false, "Dev: Build matcha binaries in the bin directory")
	flag.Parse()

	if *build {
		buildBinaries()
		os.Exit(0)
	}

	if len(*configFile) > 0 {
		viper.SetConfigFile(*configFile)
	} else {
		viper.AddConfigPath(".")
		generateConfigFile(currentDir)
		viper.SetConfigName("config")
	}

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Print(err)
		panic("Error reading yaml configuration file")
	}

	if viper.IsSet("markdown_dir_path") {
		cfg.MarkdownDirPath = viper.Get("markdown_dir_path").(string)
	} else {
		cfg.MarkdownDirPath = currentDir
	}

	feeds := viper.Get("feeds")
	if viper.IsSet("weather_latitude") {
		cfg.WeatherLatitude = viper.Get("weather_latitude").(float64)
	}
	if viper.IsSet("weather_longitude") {
		cfg.WeatherLongitude = viper.Get("weather_longitude").(float64)
	}
	if viper.IsSet("markdown_file_prefix") {
		cfg.MarkdownFilePrefix = viper.Get("markdown_file_prefix").(string)
		mdPrefix = cfg.MarkdownFilePrefix
	}
	if viper.IsSet("markdown_file_suffix") {
		cfg.MarkdownFileSuffix = viper.Get("markdown_file_suffix").(string)
		mdSuffix = cfg.MarkdownFileSuffix
	}
	if viper.IsSet("openai_api_key") {
		cfg.OpenaiApiKey = viper.Get("openai_api_key").(string)
	}
	if viper.IsSet("openai_base_url") {
		cfg.OpenaiBaseURL = viper.Get("openai_base_url").(string)
	}
	if viper.IsSet("openai_model") {
		cfg.OpenaiModel = viper.Get("openai_model").(string)
	}

	cfg.SummaryFeeds = []RSS{}
	if viper.IsSet("summary_feeds") {
		summaryFeeds := viper.Get("summary_feeds")
		for _, summaryFeed := range summaryFeeds.([]any) {
			url, limit := getFeedAndLimit(summaryFeed.(string))
			feed := RSS{url: url, limit: limit, summarize: true}
			cfg.SummaryFeeds = append(cfg.SummaryFeeds, feed)
		}
	}

	if viper.IsSet("summary_article_lenght_limit") {
		cfg.SummaryArticleLengthLimit = viper.Get("summary_article_lenght_limit").(int)
	}

	for _, feed := range feeds.([]any) {
		url, limit := getFeedAndLimit(feed.(string))
		rss := RSS{url: url, limit: limit}
		cfg.Feeds = append(cfg.Feeds, rss)
	}

	if viper.IsSet("google_news_keywords") {
		cfg.GoogleNewsKeywords = viper.Get("google_news_keywords").(string)
		googleNewsKeywords := url.QueryEscape(cfg.GoogleNewsKeywords)
		if googleNewsKeywords != "" {
			googleNewsUrl := "https://news.google.com/rss/search?hl=en-US&gl=US&ceid=US%3Aen&oc=11&q=" + strings.Join(strings.Split(googleNewsKeywords, "%2C"), "%20%7C%20")
			cfg.GoogleNewsFeeds = append(cfg.GoogleNewsFeeds, RSS{url: googleNewsUrl, limit: 15})
		}
	}

	cfg.OpmlFilePath = ""
	configPath := currentDir + "/" + "config.opml"
	if _, err := os.Stat(configPath); err == nil {
		xmlContent, _ := os.ReadFile(currentDir + "/" + "config.opml")
		feeds := parseOPML(xmlContent)
		cfg.OpmlFeeds = append(cfg.OpmlFeeds, feeds...)
	}
	if len(*opmlFile) > 0 {
		xmlContent, _ := os.ReadFile(*opmlFile)
		feeds := parseOPML(xmlContent)
		cfg.OpmlFeeds = append(cfg.OpmlFeeds, feeds...)
	}
	if viper.IsSet("opml_file_path") {
		cfg.OpmlFilePath = viper.Get("opml_file_path").(string)
		xmlContent, _ := os.ReadFile(cfg.OpmlFilePath)
		feeds := parseOPML(xmlContent)
		cfg.OpmlFeeds = append(cfg.OpmlFeeds, feeds...)
	}

	cfg.Instapaper = viper.GetBool("instapaper")
	cfg.ReadingTime = viper.GetBool("reading_time")
	cfg.ShowImages = viper.GetBool("show_images")
	cfg.SunriseSunset = viper.GetBool("sunrise_sunset")
	cfg.DatabaseFilePath = viper.GetString("database_file_path")

	if !cfg.TerminalMode {
		markdown_file_name := mdPrefix + currentDate + mdSuffix + ".md"
		os.Remove(filepath.Join(cfg.MarkdownDirPath, markdown_file_name))
	}

	return cfg
}

func generateConfigFile(currentDir string) {
	configPath := currentDir + "/" + "config.yaml"
	if _, err := os.Stat(configPath); err == nil {
		// File exists, dont do anything
		return
	}
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		log.Fatal(err)
		return
	}

	if _, err := f.Write([]byte(config)); err != nil {
		log.Fatal(err)
	}
}
