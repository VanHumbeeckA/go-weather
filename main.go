package main

import (
	"net/http"
	"encoding/json"
	"strings"
	"log"
	"time"
)

func main() {



	http.HandleFunc("/hello", hello)
	http.HandleFunc("/weather/", weather)
	http.ListenAndServe(":8080", nil)
}

func weather(w http.ResponseWriter, r *http.Request) {

	mw := multiWeatherProvider{
		openWeatherMap{},
		weatherUnderground{apiKey:"079016af567656e2"},
	}
	begin := time.Now()
	city := strings.SplitN(r.URL.Path, "/", 3)[2]

	temp, err := mw.temperature(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city": city,
		"temp": temp,
		"took": time.Since(begin).String(),
	})
}

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello!"))
}

type weatherData struct {
	Name string `json:"name"`
	Main struct{
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

type weatherProvider interface {
	temperature(city string) (float64, error)
}

type weatherUnderground struct {
	apiKey string
}

type openWeatherMap struct {

}

type multiWeatherProvider []weatherProvider

func (w multiWeatherProvider) temperature(city string) (float64, error) {


	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))



	for _, provider := range w {
		go func(p weatherProvider) {
			k, err := p.temperature(city)
			if err != nil {
				errs <- err
				return
			}
			temps <- k
		}(provider)
	}

	sum := 0.0
	for i := 0; i < len(w); i++ {
		select {
		case temp := <-temps:
			sum += temp
		case err := <-errs :
			return 0, err
		}
	}

	return sum / float64(len(w)), nil
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	var api_key string = "185ed32dcf22910c1e9e349413b37b57"
	//var api_key_2 string = "079016af567656e2"
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID="+ api_key + "&q=" + city)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d weatherData

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)

	return d.Main.Kelvin, nil
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/" + city + ".json")
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct{
		Observation struct{
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	kelvin := d.Observation.Celsius + 273.15
	log.Printf("weatherUnderground: %s, %.2f", city, kelvin)
	return kelvin, nil
}

