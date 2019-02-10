package httplistener

import (
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/spf13/viper"
	"gopkg.in/go-playground/webhooks.v5/github"
	"net/http"
	"strings"
)

func interestingIssueAction(action string) bool {
	switch action {
	case "opened", "closed", "reopened":
		return true
	}
	return false
}

func (hl *HTTPListener) githubHandler(w http.ResponseWriter, request *http.Request) {
	if request.Method != "POST" {
		http.NotFound(w, request)
		return
	}

	hook, err := github.New(github.Options.Secret(viper.GetString("http.listeners.github.secret")))

	var err2 error

	if err != nil {
		return
	}

	// All valid events we want to receive need to be listed here.
	payload, err := hook.Parse(request,
		github.ReleaseEvent, github.PushEvent, github.IssuesEvent, github.IssueCommentEvent,
		github.PullRequestEvent)

	if err != nil {
		// This usually happens because we've received an event we don't need to handle.
		log.Warningf("Error parsing github webhook: %s", err)
		return
	}

	msgs := []string{}
	tmsgs := []string{}
	repo := ""
	send := false

	switch payload.(type) {
	case github.ReleasePayload:
		pl := payload.(github.ReleasePayload)
		if pl.Action == "published" {
			send = true
			msgs, err = hl.renderTemplate("github.release.irc", payload)
			repo = pl.Repository.Name
		}
	case github.PushPayload:
		pl := payload.(github.PushPayload)
		send = true
		msgs, err = hl.renderTemplate("github.push.irc", payload)
		tmsgs, err2 = hl.renderTemplate("github.push.twitter", payload)
		repo = pl.Repository.Name
	case github.IssuesPayload:
		pl := payload.(github.IssuesPayload)
		if interestingIssueAction(pl.Action) {
			send = true
			msgs, err = hl.renderTemplate("github.issue.irc", payload)
			repo = pl.Repository.Name
		}
	case github.IssueCommentPayload:
		pl := payload.(github.IssueCommentPayload)
		if pl.Action == "created" {
			send = true
			msgs, err = hl.renderTemplate("github.issuecomment.irc", payload)
			repo = pl.Repository.Name
		}
	case github.PullRequestPayload:
		pl := payload.(github.PullRequestPayload)
		if interestingIssueAction(pl.Action) {
			send = true
			msgs, err = hl.renderTemplate("github.pullrequest.irc", payload)
			repo = pl.Repository.Name
		}
	}

	if err != nil {
		log.Errorf("Error rendering GitHub event template: %s", err)
		return
	}

	if err2 != nil {
		log.Errorf("Error rendering GitHub event template: %s", err)
		return
	}

	if send {
		prevtweet := int64(0)
		repo = strings.ToLower(repo)
		channel := viper.GetString("http.listeners.github.default_channel")
		if channel == "" {
			channel = viper.GetString(fmt.Sprintf("http.listeners.github.repositories.%s", repo))
		}

		if channel == "" {
			log.Infof("%s GitHub event for unrecognised repository %s", request.RemoteAddr, repo)
			return
		}

		log.Infof("%s [%s -> %s] GitHub event received", request.RemoteAddr, repo, channel)
		for _, msg := range msgs {
			hl.irc.Privmsgf(channel, msg)
		}
		for _, msg := range tmsgs {
			if msg == "" { continue }
			params := twitter.StatusUpdateParams{InReplyToStatusID: prevtweet}
			log.Infof("msg=%q", msg)
			tweet, resp, err := hl.twitter.Statuses.Update(msg, &params)
			prevtweet = tweet.ID
			log.Infof("tweet=%s resp=%s err=%s", tweet, resp, err)
		}
	}
}
