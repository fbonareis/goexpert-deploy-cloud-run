package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	baseURLweatherAPI = "http://api.weatherapi.com/v1"
	baseURLviaCEP     = "http://viacep.com.br"
)

var (
	ErrInvalidZipCode    = errors.New("invalid zipcode")
	ErrCanNotFindZipCode = errors.New("can not find zipcode")
)

type (
	LocationResponse struct {
		City string `json:"localidade"`
		Erro bool   `json:"erro"`
	}
	WeatherResponse struct {
		Current struct {
			TempC float64 `json:"temp_c"`
			TempF float64 `json:"temp_f"`
		} `json:"current"`
	}
	LocationWeatherResponse struct {
		TempC float64 `json:"temp_C"`
		TempF float64 `json:"temp_F"`
		TempK float64 `json:"temp_K"`
	}
)

func (w *WeatherResponse) GetTempF() float64 {
	return roundFloat(w.Current.TempC*1.8+32, 2)
}
func (w *WeatherResponse) GetTempK() float64 {
	return roundFloat(w.Current.TempC+273, 2)
}

type (
	ZipCodeService interface {
		GetLocation(zipCode string) (*LocationResponse, error)
	}
	WeatherService interface {
		GetWeatherFromCity(city string) (*WeatherResponse, error)
	}
)

type RealZipCodeService struct{}

type RealWeatherService struct{}

func (s *RealZipCodeService) GetLocation(zipCode string) (*LocationResponse, error) {
	if len(zipCode) != 8 {
		return nil, ErrInvalidZipCode
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/ws/%s/json", baseURLviaCEP, zipCode), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var l LocationResponse
	if err = json.Unmarshal(body, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, e := transform.String(t, s)
	if e != nil {
		panic(e)
	}
	return output
}

func (s *RealWeatherService) GetWeatherFromCity(city string) (*WeatherResponse, error) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return nil, errors.New("weather api key not found")
	}

	c := strings.ReplaceAll(removeAccents(city), " ", "%20")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/current.json?key=%s&q=%s&aqi=no", baseURLweatherAPI, apiKey, c), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var w WeatherResponse
	if err = json.Unmarshal(body, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func main() {
	zipCodeService := &RealZipCodeService{}
	weatherService := &RealWeatherService{}
	http.HandleFunc("/weather", createHandler(zipCodeService, weatherService))
	http.ListenAndServe(":8080", nil)
}

func createHandler(zipCodeService ZipCodeService, weatherService WeatherService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		zipCode := r.URL.Query().Get("zipcode")
		response, err := GetWeather(zipCodeService, weatherService, zipCode)

		if err != nil {
			handleError(w, err)
			return
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func handleError(w http.ResponseWriter, err error) {
	var status int

	switch {
	case errors.Is(err, ErrInvalidZipCode):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, ErrCanNotFindZipCode):
		status = http.StatusNotFound
	default:
		status = http.StatusInternalServerError
	}

	http.Error(w, err.Error(), status)
}

func GetWeather(zipCodeService ZipCodeService, weatherService WeatherService, zipCode string) (*LocationWeatherResponse, error) {
	location, err := zipCodeService.GetLocation(zipCode)
	if err != nil {
		return nil, err
	}
	if location.Erro {
		return nil, ErrCanNotFindZipCode
	}

	weather, err := weatherService.GetWeatherFromCity(location.City)
	if err != nil {
		return nil, err
	}

	return &LocationWeatherResponse{
		TempC: weather.Current.TempC,
		TempF: weather.GetTempF(),
		TempK: weather.GetTempK(),
	}, nil
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
