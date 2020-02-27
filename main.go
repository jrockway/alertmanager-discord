package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/jrockway/opinionated-server/client"
	"github.com/jrockway/opinionated-server/server"
	"github.com/prometheus/alertmanager/template"
	"go.uber.org/zap"
)

type discordOut struct {
	Content string `json:"content"`
}

type appflags struct {
	WebhookURL string `long:"webhook.url" env:"DISCORD_WEBHOOK" description:"Your Discord webhook URL." required:"true"`
}

var cl = &http.Client{
	Transport: client.WrapRoundTripper(nil),
}

func sendOneAlert(ctx context.Context, url string, alerts *template.Data, alert template.Alert) error {
	discordMsg := discordOut{}
	alertJson, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("formatting alert: %w", err)
	}
	discordMsg.Content = fmt.Sprintf("```%s```", alertJson)
	body, err := json.Marshal(discordMsg)
	if err != nil {
		return fmt.Errorf("marshaling discord message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}
	req.Header.Add("content-type", "application/json")
	res, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("executing http request: %w", err)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("non-200 response from discord: %d %s", res.StatusCode, res.Status)
	}
	return nil
}

func makeWebhookHandler(url string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := ctxzap.Extract(req.Context())
		b, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			logger.Error("problem reading request body", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		alerts := new(template.Data)
		if err := json.Unmarshal(b, &alerts); err != nil {
			logger.Error("problem unmarshalling alerts", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if alerts.Alerts == nil || len(alerts.Alerts) < 1 {
			logger.Error("no alerts provided", zap.ByteString("body", b))
			http.Error(w, "no alerts provided", http.StatusBadRequest)
			return
		}

		var errored bool
		for _, alert := range alerts.Alerts {
			if err := sendOneAlert(req.Context(), url, alerts, alert); err != nil {
				logger.Error("problem sending request", zap.Error(err))
				errored = true
			}
		}
		if errored {
			http.Error(w, "problem sending alert; see the logs", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func main() {
	server.AppName = "alertmanager-discord"

	f := new(appflags)
	server.AddFlagGroup("alertmanager-discord", f)
	server.Setup()
	zap.L().Info("webhook", zap.String("url", f.WebhookURL))
	server.SetHTTPHandler(makeWebhookHandler(f.WebhookURL))
	server.SetStartupCallback(func(server.Info) {
		ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		alertData := &template.Data{
			Alerts: template.Alerts{
				template.Alert{
					Status: "Webhook receiver starting up.",
				},
			},
		}
		if err := sendOneAlert(ctx, f.WebhookURL, alertData, alertData.Alerts[0]); err != nil {
			// We don't kill the program here, because Discord could be down.  But at
			// least you'll see something in the logs before a real alert if your
			// webhook is broken.
			zap.L().Error("problem sending initial alert", zap.Error(err))
		}
	})
	server.ListenAndServe()
}
