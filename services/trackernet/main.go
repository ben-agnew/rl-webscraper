package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	metricsMw "github.com/slok/go-http-metrics/middleware"
	metricsMwStd "github.com/slok/go-http-metrics/middleware/std"
	"github.com/tkanos/gonfig"
	"github.com/yannismate/yannismate-api/libs/cache"
	"github.com/yannismate/yannismate-api/libs/httplog"
	"github.com/yannismate/yannismate-api/libs/rest/trackernet"
)

var configuration = Configuration{}
var redisCache cache.Cache

func main() {
	err := gonfig.GetConf("config.json", &configuration)
	if err != nil {
		log.WithField("event", "load_config").Fatal(err)
		return
	}

	redisCache = cache.NewCache(configuration.Cache.RedisUrl)

	mdlw := metricsMw.New(metricsMw.Config{
		Recorder: metrics.NewRecorder(metrics.Config{
			DurationBuckets: []float64{1, 2.5, 5, 10, 30, 60, 120, 300, 600},
		}),
	})

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/rank", metricsMwStd.Handler("rank", mdlw, httplog.WithLogging(rankHandler())))
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.WithField("event", "start_server").Fatal(err)
	}
}

var platforms = map[string]string{trackernet.Steam: "steam", trackernet.Epic: "epic", trackernet.PS: "psn", trackernet.Xbox: "xbl"}

func rankHandler() http.Handler {
	fn := func(rw http.ResponseWriter, r *http.Request) {

		platform, ok := platforms[strings.ToLower(r.URL.Query().Get("platform"))]
		if !ok {
			rw.WriteHeader(400)
			return
		}

		user := r.URL.Query().Get("user")
		if user == "" {
			rw.WriteHeader(400)
			return
		}

		cacheRes, err := redisCache.Get(platform + ":" + user)
		if err == nil {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(cacheRes))
			return
		}

		rankRes, err := GetRanks(platform, user)
		if err != nil {
			if _, ok := err.(*TggError); ok {

				rw.WriteHeader(404)
				return
			} else {
				log.WithField("event", "get_ranks").Warn(err)
				rw.WriteHeader(500)
				return
			}
		}

		jData, err := json.Marshal(rankRes)
		if err != nil {
			log.WithField("event", "json_encode").Error(err)
			rw.WriteHeader(500)
			return
		}

		err = redisCache.SetWithTtl(platform+":"+user, string(jData), time.Second*time.Duration(configuration.Cache.TtlSeconds))
		if err != nil {
			log.WithField("event", "cache_set").Error(err)
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(200)
		_, err = rw.Write(jData)
		if err != nil {
			log.WithField("event", "write_response").Error(err)
		}

	}
	return http.HandlerFunc(fn)
}
