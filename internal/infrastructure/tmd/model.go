package tmd

// ForecastResponse is the generic envelope returned by the TMD NWP API.
type ForecastResponse struct {
	Forecasts []Forecast `json:"forecasts"`
}

// Forecast represents a single location's forecast data.
type Forecast struct {
	Location Location        `json:"location"`
	Data     []ForecastEntry `json:"data"`
}

// Location holds geographic information for a forecast point.
type Location struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Province string  `json:"province"`
	Amphoe   string  `json:"amphoe"`
	Tambon   string  `json:"tambon"`
}

// ForecastEntry is a single time-step within a forecast.
type ForecastEntry struct {
	Time string  `json:"time"`
	Data DataSet `json:"data"`
}

// DataSet holds the meteorological variables for a single time-step.
type DataSet struct {
	TC    *float64 `json:"tc,omitempty"`
	RH    *float64 `json:"rh,omitempty"`
	Slp   *float64 `json:"slp,omitempty"`
	Rain  *float64 `json:"rain,omitempty"`
	WS10m *float64 `json:"ws10m,omitempty"`
	WD10m *float64 `json:"wd10m,omitempty"`
	Cond  *int     `json:"cond,omitempty"`
	TCMax *float64 `json:"tc_max,omitempty"`
	TCMin *float64 `json:"tc_min,omitempty"`
}

// TargetLocation defines a location to be fetched and cached.
type TargetLocation struct {
	Code     string
	Lat      float64
	Lon      float64
	Province string // Thai province name for Area endpoint
	Amphoe   string // Thai amphoe name for Area endpoint (optional)
	Tambon   string // Thai tambon name for Area endpoint (optional)
}
