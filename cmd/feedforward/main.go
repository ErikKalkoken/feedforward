package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedforward/internal/app"
	"github.com/ErikKalkoken/feedforward/internal/app/service"
	"github.com/ErikKalkoken/feedforward/internal/app/storage"
)

const (
	configFilename = "config.toml"
	dbFileName     = "feedforward.db"
)

// Current version need to be injected via ldflags
var Version = "0.0.0"

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfgPathFlag := flag.String("config", ".", "path to configuration file")
	dbPathFlag := flag.String("db", ".", "path to database file")
	versionFlag := flag.Bool("v", false, "show version")
	showDBFlag := flag.Bool("show-db", false, "show contents of database")
	flag.Usage = myUsage
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	p := filepath.Join(*cfgPathFlag, configFilename)
	cfg, err := app.ReadConfig(p)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	slog.SetLogLoggerLevel(cfg.App.LoggerLevel())
	p = filepath.Join(*dbPathFlag, dbFileName)
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		log.Fatalf("DB init failed: %s", err)
	}
	if *showDBFlag {
		printDBContent(st)
		os.Exit(0)
	}
	a := service.New(st, cfg, realtime{})
	a.Start()
	defer a.Close()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func printDBContent(st *storage.Storage) {
	feeds, err := st.ListFeeds()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("feeds (%d)\n", len(feeds))
	for _, f := range feeds {
		items, err := st.ListItems(f)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("    %s (%d)\n", f, len(items))
		for _, i := range items {
			fmt.Printf("        %s | %s\n", i.Published, i.ID)
		}
	}
}

// myUsage writes a custom usage message to configured output stream.
func myUsage() {
	s := "Usage: feedforward [options]:\n\n" +
		"A service for forwarding RSS and Atom feeds to Discord webhooks.\n" +
		"For more information please see: https://github.com/ErikKalkoken/feedforward\n\n" +
		"Options:\n"
	fmt.Fprint(flag.CommandLine.Output(), s)
	flag.PrintDefaults()
}
