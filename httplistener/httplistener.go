package httplistener

import (
	"github.com/dghubble/go-twitter/twitter"
	"github.com/irccloud/go-ircevent"
	"github.com/juju/loggo"
	"github.com/spf13/viper"
	"net/http"
	"text/template"
)

var log = loggo.GetLogger("HTTPListener")

type HTTPListener struct {
	http    http.Server
	irc     *irc.Connection
	twitter *twitter.Client
	tpls    *template.Template
}

func New(irc *irc.Connection, twittercon *twitter.Client) (*HTTPListener, error) {
	hl := HTTPListener{}
	hl.irc = irc
	hl.twitter = twittercon
	hl.http = http.Server{Addr: viper.GetString("http.listen")}
	hl.tpls = parseTemplates()
	log.Infof("Listening for HTTP requests on %s", viper.GetString("http.listen"))

	mux := http.NewServeMux()

	if viper.GetBool("http.listeners.generic") {
		log.Infof("Listening for HTTP POST requests at /send")
		mux.HandleFunc("/send", hl.genericHandler)
	}

	if viper.IsSet("http.listeners.grafana") {
		log.Infof("Listening for Grafana webhooks at /grafana")
		mux.HandleFunc("/grafana", hl.grafanaAlertHandler)
	}

	if viper.IsSet("http.listeners.github") {
		log.Infof("Listening for GitHub webhooks at /github")
		mux.HandleFunc("/github", hl.githubHandler)
	}

	hl.http.Handler = mux
	if viper.GetBool("http.tls") {
		go hl.http.ListenAndServeTLS(viper.GetString("http.tls_cert"), viper.GetString("http.tls_key"))
	} else {
		go hl.http.ListenAndServe()
	}
	return &hl, nil
}
