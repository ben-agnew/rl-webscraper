package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yannismate/yannismate-api/libs/rest/trackernet"
	"github.com/yannismate/yannismate-api/libs/rest/webscraper"
)

var httpClient = http.Client{
	Timeout: time.Second * 10,
}

func GetRanks(platform string, user string) (*trackernet.GetRankResponse, error) {

	requestUrl := configuration.TrackerNet.BaseUrl + "/" + platform + "/" + strings.Replace(url.QueryEscape(user), "+", "%20", -1)
	req, err := http.NewRequest("GET", configuration.ScraperUrl+"?url="+url.QueryEscape(requestUrl), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "yannismate-api/services/trackernet")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	scraperRes := webscraper.GetScrapeResponse{}
	err = json.Unmarshal(body, &scraperRes)
	if err != nil {
		return nil, err
	}

	tggRes := TggResponse{}

	err = json.Unmarshal([]byte(scraperRes.Content), &tggRes)
	if err != nil {
		return nil, err
	}

	if len(tggRes.Errors) > 0 {
		return nil, &TggError{}
	}

	rankings := make([]trackernet.Ranking, 0)
	for _, s := range tggRes.Data.Segments {
		if s.Type == "playlist" {
			ranking := s.toRanking()
			if ranking != nil {
				rankings = append(rankings, *ranking)
			}
		}
	}
	displayName := tggRes.Data.PlatformInfo.PlatformUserHandle

	return &trackernet.GetRankResponse{DisplayName: displayName, Rankings: rankings}, nil
}

type TggResponse struct {
	Errors []map[string]interface{} `json:"errors"`
	Data   TggData                  `json:"data"`
}

type TggData struct {
	PlatformInfo TggPlatformInfo `json:"platformInfo"`
	Segments     []TggSegment    `json:"segments"`
}

type TggPlatformInfo struct {
	PlatformUserHandle string `json:"platformUserHandle"`
}

type TggSegment struct {
	Type     string          `json:"type"`
	Metadata TggSegmentMeta  `json:"metadata"`
	Stats    TggSegmentStats `json:"stats"`
}

type TggSegmentMeta struct {
	Name string `json:"name"`
}

type TggSegmentStats struct {
	Tier      TggTier       `json:"tier"`
	Division  TggDivision   `json:"division"`
	Rating    TggStatsValue `json:"rating"`
	WinStreak TggWinStreak  `json:"winStreak"`
}

type TggTier struct {
	Value        int             `json:"value"`
	DisplayValue string          `json:"displayValue"`
	MetaData     TggTierMetaData `json:"metadata"`
}

type TggTierMetaData struct {
	Name string `json:"name"`
}

type TggDivision struct {
	Value        int                 `json:"value"`
	DisplayValue string              `json:"displayValue"`
	MetaData     TggDivisionMetaData `json:"metadata"`
}

type TggDivisionMetaData struct {
	Name      string `json:"name"`
	DeltaUp   int    `json:"deltaUp"`
	DeltaDown int    `json:"deltaDown"`
}

type TggWinStreak struct {
	Value        int    `json:"value"`
	DisplayValue string `json:"displayValue"`
}

type TggStatsValue struct {
	Value int `json:"value"`
}

type TggError struct{}

func (t TggError) Error() string {
	return "tracker.gg API returned error object"
}

var playlists = map[string]trackernet.Playlist{"Un-Ranked": trackernet.Unranked, "Ranked Duel 1v1": trackernet.Ranked1v1,
	"Ranked Doubles 2v2": trackernet.Ranked2v2, "Ranked Standard 3v3": trackernet.Ranked3v3, "Hoops": trackernet.Hoops,
	"Rumble": trackernet.Rumble, "Dropshot": trackernet.Dropshot, "Snowday": trackernet.Snowday, "Tournament Matches": trackernet.Tournaments}

func (seg *TggSegment) toRanking() *trackernet.Ranking {

	playlist, ok := playlists[seg.Metadata.Name]
	if !ok {
		return nil
	}

	return &trackernet.Ranking{
		Playlist:     playlist,
		Mmr:          seg.Stats.Rating.Value,
		Rank:         seg.Stats.Tier.Value,
		Division:     seg.Stats.Division.Value,
		WinStreak:    seg.Stats.WinStreak.DisplayValue,
		RankName:     seg.Stats.Tier.MetaData.Name,
		DivisionName: seg.Stats.Division.MetaData.Name,
		DeltaUp:      seg.Stats.Division.MetaData.DeltaUp,
		DeltaDown:    seg.Stats.Division.MetaData.DeltaDown,
	}
}
