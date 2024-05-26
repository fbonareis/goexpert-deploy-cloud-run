package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockZipCodeService struct {
	mock.Mock
}

func (m *MockZipCodeService) GetLocation(zipCode string) (*LocationResponse, error) {
	args := m.Called(zipCode)
	return args.Get(0).(*LocationResponse), args.Error(1)
}

type MockWeatherService struct {
	mock.Mock
}

func (m *MockWeatherService) GetWeatherFromCity(city string) (*WeatherResponse, error) {
	args := m.Called(city)
	return args.Get(0).(*WeatherResponse), args.Error(1)
}

func TestGetLocation(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	testZipCode := "12345678"
	expectedLocation := &LocationResponse{City: "TestCity", Erro: false}
	mockZipService.On("GetLocation", testZipCode).Return(expectedLocation, nil)

	location, err := mockZipService.GetLocation(testZipCode)
	assert.NoError(t, err)
	assert.Equal(t, expectedLocation, location)
	mockZipService.AssertExpectations(t)
}

func TestGetWeatherFromCity(t *testing.T) {
	mockWeatherService := new(MockWeatherService)
	testCity := "TestCity"
	expectedWeather := &WeatherResponse{}
	expectedWeather.Current.TempC = 25.0
	expectedWeather.Current.TempF = 77.0
	mockWeatherService.On("GetWeatherFromCity", testCity).Return(expectedWeather, nil)

	weather, err := mockWeatherService.GetWeatherFromCity(testCity)
	assert.NoError(t, err)
	assert.Equal(t, expectedWeather, weather)
	mockWeatherService.AssertExpectations(t)
}

func TestGetWeather(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	mockWeatherService := new(MockWeatherService)
	testZipCode := "12345678"
	testCity := "TestCity"
	expectedLocation := &LocationResponse{City: testCity, Erro: false}
	expectedWeather := &WeatherResponse{}
	expectedWeather.Current.TempC = 25.0
	expectedWeather.Current.TempF = 77.0

	mockZipService.On("GetLocation", testZipCode).Return(expectedLocation, nil)
	mockWeatherService.On("GetWeatherFromCity", testCity).Return(expectedWeather, nil)

	expectedLocationWeather := &LocationWeatherResponse{
		TempC: 25.0,
		TempF: 77.0,
		TempK: 298,
	}

	locationWeather, err := GetWeather(mockZipService, mockWeatherService, testZipCode)
	assert.NoError(t, err)
	assert.Equal(t, expectedLocationWeather, locationWeather)
	mockZipService.AssertExpectations(t)
	mockWeatherService.AssertExpectations(t)
}

func TestGetWeatherInvalidZipCode(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	mockWeatherService := new(MockWeatherService)
	invalidZipCode := "123"

	mockZipService.On("GetLocation", invalidZipCode).Return(&LocationResponse{}, ErrInvalidZipCode)

	_, err := GetWeather(mockZipService, mockWeatherService, invalidZipCode)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidZipCode, err)
	mockZipService.AssertExpectations(t)
}

func TestCreateHandler_Endpoint_Success(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	mockWeatherService := new(MockWeatherService)
	testZipCode := "12345678"
	testCity := "TestCity"
	expectedLocation := &LocationResponse{City: testCity, Erro: false}
	expectedWeather := &WeatherResponse{}
	expectedWeather.Current.TempC = 25.0
	expectedWeather.Current.TempF = 77.0

	mockZipService.On("GetLocation", testZipCode).Return(expectedLocation, nil)
	mockWeatherService.On("GetWeatherFromCity", testCity).Return(expectedWeather, nil)

	req, err := http.NewRequest("GET", "/weather?zipcode=12345678", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := createHandler(mockZipService, mockWeatherService)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response LocationWeatherResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	expectedResponse := LocationWeatherResponse{
		TempC: 25.0,
		TempF: 77.0,
		TempK: 298,
	}
	assert.Equal(t, expectedResponse, response)

	mockZipService.AssertExpectations(t)
	mockWeatherService.AssertExpectations(t)
}

func TestCreateHandler_Endpoint_NotFound(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	mockWeatherService := new(MockWeatherService)
	testZipCode := "12345678"
	expectedLocation := &LocationResponse{Erro: true}
	mockZipService.On("GetLocation", testZipCode).Return(expectedLocation, nil)

	req, err := http.NewRequest("GET", fmt.Sprintf("/weather?zipcode=%s", testZipCode), nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := createHandler(mockZipService, mockWeatherService)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "can not find zipcode")
	mockZipService.AssertExpectations(t)
	mockWeatherService.AssertExpectations(t)
}

func TestCreateHandler_Endpoint_UnprocessableContent(t *testing.T) {
	mockZipService := new(MockZipCodeService)
	mockWeatherService := new(MockWeatherService)
	testZipCode := "123"
	mockZipService.On("GetLocation", testZipCode).Return(&LocationResponse{}, ErrInvalidZipCode)

	req, err := http.NewRequest("GET", fmt.Sprintf("/weather?zipcode=%s", testZipCode), nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := createHandler(mockZipService, mockWeatherService)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid zipcode")
	mockZipService.AssertExpectations(t)
	mockWeatherService.AssertExpectations(t)
}
