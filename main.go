package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/net/context"
)

const (
	bucketFeeds = "feeds"
	oldest      = 24 * time.Hour
)

type configMain struct {
	Feeds    []configFeed
	Webhooks []configWebhook
}

type configFeed struct {
	Name    string `toml:"name"`
	URL     string `toml:"url"`
	Webhook string `toml:"webhook"`
}

type configWebhook struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

type webhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds,omitempty"`
}

type embed struct {
	Description string `json:"description,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
}

var converter = md.NewConverter("", true, nil)

type app struct {
	db       *bolt.DB
	config   configMain
	webhooks map[string]string
}

func main() {
	var config configMain
	if _, err := toml.DecodeFile("config.toml", &config); err != nil {
		log.Fatal(err)
	}

	db, err := bolt.Open("rssfeed.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
		return err
	}); err != nil {
		log.Fatal(err)
	}
	app := NewApp(db, config)
	ctx, cancel := context.WithCancel(context.Background())
	go app.run(ctx)

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	cancel()
}

func NewApp(db *bolt.DB, config configMain) *app {
	webhooksUsed := make(map[string]bool)
	webhooks := make(map[string]string)
	for _, x := range config.Webhooks {
		webhooks[x.Name] = x.URL
	}
	for _, x := range config.Feeds {
		_, ok := webhooks[x.Webhook]
		if !ok {
			log.Fatalf("Config error: Invalid webhook name \"%s\" for feed \"%s\"", x.Webhook, x.Name)
		}
		webhooksUsed[x.Webhook] = true
	}
	for k, v := range webhooksUsed {
		if !v {
			slog.Warn("Webhook defined, but not used", "name", k)
		}
	}
	app := &app{
		db:       db,
		config:   config,
		webhooks: webhooks,
	}
	return app
}

func (a *app) run(ctx context.Context) {
	fp := gofeed.NewParser()
	ticker := time.NewTicker(5 * time.Second)
	for {
		slog.Info("Checking feeds")
		for _, cf := range a.config.Feeds {
			select {
			case <-ctx.Done():
				return
			default:
			}
			var lastPublished time.Time
			a.db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(bucketFeeds))
				v := b.Get([]byte(cf.URL))
				if v != nil {
					t, err := time.Parse(time.RFC3339, string(v))
					if err != nil {
						slog.Error("failed to parse last published", "value", v, "error", err)
					} else {
						lastPublished = t
					}
				}
				return nil
			})
			feed, _ := fp.ParseURL(cf.URL)
			var newest time.Time
			for _, item := range feed.Items {
				if !item.PublishedParsed.After(lastPublished) {
					continue
				}
				if item.PublishedParsed.Before(time.Now().Add(-oldest)) {
					continue
				}
				if err := sendItemToWebhook(feed.Title, item, a.webhooks[cf.Webhook]); err != nil {
					slog.Error("Failed to send item", "error", "err")
				}
				if item.PublishedParsed.After(newest) {
					newest = *item.PublishedParsed
				}
			}
			if !newest.IsZero() {
				if err := a.db.Update(func(tx *bolt.Tx) error {
					b := tx.Bucket([]byte(bucketFeeds))
					err := b.Put([]byte(cf.URL), []byte(newest.Format(time.RFC3339)))
					return err
				}); err != nil {
					log.Fatal(err)
				}
			}
		}
		slog.Info("Completed checking feeds")
		select {
		case <-ctx.Done():
			return
		default:
			<-ticker.C
		}
	}
}

func sendItemToWebhook(title string, item *gofeed.Item, url string) error {
	description, err := converter.ConvertString(item.Content)
	if err != nil {
		return fmt.Errorf("failed to parse content to markdown: %w", err)
	}
	payload := webhookPayload{
		Content: title,
		Embeds: []embed{{
			Description: description,
			Timestamp:   item.PublishedParsed.Format(time.RFC3339),
			Title:       item.Title,
			URL:         item.Link,
		}},
	}
	if err := sendToWebhook(payload, url); err != nil {
		return fmt.Errorf("failed to send to webhook: %w", err)
	}
	return nil
}

func sendToWebhook(payload webhookPayload, url string) error {
	time.Sleep(1 * time.Second)
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("response", "url", url, "status", resp.Status, "headers", resp.Header, "body", string(body))
	slog.Info("message posted", "webhook", url, "status", resp.Status)
	return nil
}
