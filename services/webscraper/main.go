package main

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/tebeka/selenium"
	"github.com/tkanos/gonfig"
	"github.com/yannismate/yannismate-api/libs/httplog"
	"github.com/yannismate/yannismate-api/libs/rest/webscraper"
	"net/http"
	"time"
)

var configuration = Configuration{}

func main() {

	err := gonfig.GetConf("config.json", &configuration)
	if err != nil {
		log.WithField("event", "load_config").Fatal(err)
		return
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/scrape", httplog.WithLogging(scrapeHandler()))
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.WithField("event", "start_server").Fatal(err)
	}
}

var selCaps = selenium.Capabilities{"browserName": "chrome"}

func scrapeHandler() http.Handler {
	fn := func(rw http.ResponseWriter, r *http.Request) {

		url := r.URL.Query().Get("url")
		if url == "" {
			rw.WriteHeader(400)
			return
		}

		remote, err := selenium.NewRemote(selCaps, configuration.Selenium.Url)
		if err != nil {
			log.WithField("event", "selenium_new_remote").Error(err)
			rw.WriteHeader(500)
			metricScrapeError.Inc()
			return
		}

		defer remote.Quit()

		err = remote.SetPageLoadTimeout(time.Millisecond * time.Duration(configuration.Selenium.PageLoadTimeout))
		if err != nil {
			log.WithField("event", "selenium_set_timeout").Error(err)
			rw.WriteHeader(500)
			metricScrapeError.Inc()
			return
		}

		err = remote.Get(url)
		if err != nil {
			log.WithField("event", "selenium_get").Error(err)
			rw.WriteHeader(500)
			metricScrapeError.Inc()
			return
		}

		element, err := remote.FindElement(selenium.ByTagName, "pre")
		if err != nil {
			src, err := remote.PageSource()
			if err != nil {
				log.WithField("event", "page_get_source").Error(err)
				rw.WriteHeader(500)
				metricScrapeError.Inc()
				return
			}
			jData, err := json.Marshal(webscraper.GetScrapeResponse{Content: src})
			if err != nil {
				log.WithField("event", "json_encode").Error(err)
				rw.WriteHeader(500)
				metricScrapeError.Inc()
				return
			}

			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			metricScrapeSuccess.Inc()
			_, err = rw.Write(jData)
			if err != nil {
				log.WithField("event", "write_response").Error(err)
			}
			return
		}
		text, err := element.Text()
		if err != nil {
			log.WithField("event", "element_get_text").Error(err)
			rw.WriteHeader(500)
			metricScrapeError.Inc()
			return
		}

		jData, err := json.Marshal(webscraper.GetScrapeResponse{Content: text})
		if err != nil {
			log.WithField("event", "json_encode").Error(err)
			rw.WriteHeader(500)
			metricScrapeError.Inc()
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(200)
		metricScrapeSuccess.Inc()
		_, err = rw.Write(jData)
		if err != nil {
			log.WithField("event", "write_response").Error(err)
		}
	}
	return http.HandlerFunc(fn)
}
