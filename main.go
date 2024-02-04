package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var opts struct {
	Listen                 string `short:"l" long:"listen" description:"Listen address" value-name:"[ADDR]:PORT" default:":9550"`
	MetricsPath            string `short:"m" long:"metrics-path" description:"Metrics path" value-name:"PATH" default:"/scrape"`
	V2RayEndpoint          string `short:"e" long:"v2ray-endpoint" description:"V2Ray API endpoint" value-name:"HOST:PORT" default:"127.0.0.1:8080"`
	ScrapeTimeoutInSeconds int64  `short:"t" long:"scrape-timeout" description:"The timeout in seconds for every individual scrape" value-name:"N" default:"3"`
	BasicAuthUsername      string `short:"u" long:"basic-auth-username" description:"Basic Auth username" value-name:"USERNAME"`
	BasicAuthPassword      string `short:"p" long:"basic-auth-password" description:"Basic Auth password" value-name:"PASSWORD"`
	Version                bool   `long:"version" description:"Display the version and exit"`
}

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

var exporter *Exporter

func scrapeHandler(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(
		exporter.registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError},
	).ServeHTTP(w, r)
}

func main() {
	var err error
	if _, err = flags.Parse(&opts); err != nil {
		os.Exit(0)
	}

	fmt.Printf("V2Ray Exporter %v-%v (built %v)\n", buildVersion, buildCommit, buildDate)

	if opts.Version {
		os.Exit(0)
	}

	scrapeTimeout := time.Duration(opts.ScrapeTimeoutInSeconds) * time.Second
	exporter, err = NewExporter(opts.V2RayEndpoint, scrapeTimeout)
	if err != nil {
		os.Exit(1)
	}

	// Check if both Basic Auth username and password are provided
	if opts.BasicAuthUsername != "" && opts.BasicAuthPassword != "" {
		http.HandleFunc(opts.MetricsPath, basicAuth(scrapeHandler, opts.BasicAuthUsername, opts.BasicAuthPassword))
	} else {
		http.HandleFunc(opts.MetricsPath, scrapeHandler)
	}
	logrus.Infof("Server is ready to handle incoming scrape requests.")
	logrus.Fatal(http.ListenAndServe(opts.Listen, nil))

	defer exporter.conn.Close()
}

// basicAuth is a middleware function to provide basic authentication.
func basicAuth(next http.HandlerFunc, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		if !ok || user != username || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("401 Unauthorized\n"))
			return
		}

		next.ServeHTTP(w, r)
	}
}
