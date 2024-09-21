// Package remoteservice contains the logic for communicating between cli and server process
package remote

import (
	"cmp"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/consoletable"
	"github.com/ErikKalkoken/feedhook/internal/dhook"
)

type EmptyArgs struct{}

type SendPingArgs struct {
	Name string
}

// RemoteService is a service for providing remote access to the app via RPC.
type RemoteService struct {
	cfg    app.MyConfig
	client *dhook.Client
	d      *dispatcher.Dispatcher
	st     *storage.Storage
}

func NewRemoteService(d *dispatcher.Dispatcher, st *storage.Storage, cfg app.MyConfig) *RemoteService {
	client := dhook.NewClient(http.DefaultClient)
	x := &RemoteService{
		cfg:    cfg,
		client: client,
		d:      d,
		st:     st,
	}
	return x
}

func (s *RemoteService) Statistics(args *EmptyArgs, reply *string) error {
	out := &strings.Builder{}
	// Feed stats
	feedsTable := consoletable.New("Feeds", 6)
	feedsTable.Target = out
	feedsTable.AddRow([]any{"Name", "Enabled", "Webhooks", "Received", "Last", "Errors"})
	slices.SortFunc(s.cfg.Feeds, func(a, b app.ConfigFeed) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cf := range s.cfg.Feeds {
		o, err := s.st.GetFeedStats(cf.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		feedsTable.AddRow([]any{o.Name, !cf.Disabled, cf.Webhooks, o.ReceivedCount, o.ReceivedLast, o.ErrorCount})
	}
	feedsTable.Print()
	fmt.Fprintln(out)
	// Webhook stats
	whTable := consoletable.New("Webhooks", 5)
	whTable.Target = out
	whTable.AddRow([]any{"Name", "Queued", "Sent", "Last", "Errors"})
	slices.SortFunc(s.cfg.Webhooks, func(a, b app.ConfigWebhook) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cw := range s.cfg.Webhooks {
		o, err := s.st.GetWebhookStats(cw.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		ms, err := s.d.MessengerStatus(cw.Name)
		if err != nil {
			slog.Error("Failed to fetch queue size for webhook", "webhook", cw.Name)
		}
		whTable.AddRow([]any{o.Name, ms.QueueSize, o.SentCount, o.SentLast, ms.ErrorCount})
	}
	whTable.Print()
	*reply = out.String()
	return nil
}

func (s *RemoteService) SendPing(args *SendPingArgs, reply *bool) error {
	var wh app.ConfigWebhook
	for _, w := range s.cfg.Webhooks {
		if w.Name == args.Name {
			wh = w
			break
		}
	}
	if wh.Name == "" {
		return fmt.Errorf("no webhook found with the name %s", args.Name)
	}
	dh := dhook.NewWebhook(s.client, wh.URL)
	pl := dhook.Message{Content: "Ping from feedhook"}
	return dh.Execute(pl)
}
