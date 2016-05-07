package weather

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/nlp"
	"github.com/itsabot/abot/shared/plugin"
)

type weatherJSON struct {
	Description []string
	Temp        float64
	Humidity    int
}

var p *dt.Plugin

func init() {
	rand.Seed(time.Now().UnixNano())
	trigger := &nlp.StructuredInput{
		Commands: []string{"what", "show", "tell", "is", "how"},
		Objects: []string{"weather", "temperature", "temp", "outside",
			"raining"},
	}
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_weather", trigger)
	if err != nil {
		log.Fatal(err)
	}
	p.Vocab = dt.NewVocab(
		dt.VocabHandler{
			Fn: kwGetTemp,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"what", "show", "tell", "how"},
				Objects: []string{"weather", "temperature",
					"temp", "outside"},
			},
		},
		dt.VocabHandler{
			Fn: kwGetRaining,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"tell", "is"},
				Objects:  []string{"rain"},
			},
		},
	)
	p.SM.SetStates([]dt.State{
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				if !p.SM.HasMemory(in, "city") {
					return "What city are you in?"
				}
				mem := p.SM.GetMemory(in, "city")
				p.Log.Debug(mem)
				city := &dt.City{}
				if err := json.Unmarshal(mem.Val, city); err != nil {
					p.Log.Info("failed to unmarshal memory.", err)
					return ""
				}
				return fmt.Sprintf("Are you still in %s?", city.Name)
			},
			OnInput: func(in *dt.Msg) {
				cities, _ := language.ExtractCities(p.DB, in)
				if len(cities) > 0 {
					p.SM.SetMemory(in, "city", cities[0])
				}
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return p.SM.HasMemory(in, "city"), ""
			},
		},
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				return kwGetTemp(in)
			},
			OnInput: func(in *dt.Msg) {},
			Complete: func(in *dt.Msg) (bool, string) {
				return true, ""
			},
		},
	})
}

func kwGetTemp(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
	if err != nil {
		p.Log.Info("failed to getCity.", err)
		return ""
	}
	p.SM.SetMemory(in, "city", city)
	return getWeather(city)
}

func kwGetRaining(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
	if err != nil {
		p.Log.Info("failed to getCity.", err)
		return ""
	}
	resp = getWeather(city)
	for _, w := range strings.Fields(resp) {
		if w == "rain" {
			return fmt.Sprintf("It's raining in %s right now.",
				city.Name)
		}
	}
	return fmt.Sprintf("It's not raining in %s right now.", city.Name)
}

func getCity(in *dt.Msg) (*dt.City, error) {
	cities, err := language.ExtractCities(p.DB, in)
	if err != nil && err != language.ErrNotFound {
		p.Log.Debug("couldn't extract cities")
		return nil, err
	}
	if len(cities) >= 1 {
		return &cities[0], nil
	}
	if p.SM.HasMemory(in, "city") {
		mem := p.SM.GetMemory(in, "city")
		p.Log.Debug(mem)
		city := &dt.City{}
		if err := json.Unmarshal(mem.Val, city); err != nil {
			p.Log.Info("couldn't unmarshal mem into city.", err)
			return nil, err
		}
		return city, nil
	}
	return nil, language.ErrNotFound
}

func getWeather(city *dt.City) string {
	p.Log.Debug("getting weather for city", city.Name)
	req := weatherJSON{}
	n := url.QueryEscape(city.Name)
	resp, err := http.Get("https://www.itsabot.org/api/weather/" + n)
	if err != nil {
		return ""
	}
	p.Log.Debug("decoding resp")
	if err = json.NewDecoder(resp.Body).Decode(&req); err != nil {
		return ""
	}
	p.Log.Debug("closing resp.Body")
	if err = resp.Body.Close(); err != nil {
		return ""
	}
	p.Log.Debug("got weather")
	var ret string
	if len(req.Description) == 0 {
		ret = fmt.Sprintf("It's %.f in %s right now.", req.Temp,
			city.Name)
	} else {
		ret = fmt.Sprintf("It's %.0f with %s in %s.", req.Temp,
			req.Description[0], city.Name)
	}
	return ret
}
