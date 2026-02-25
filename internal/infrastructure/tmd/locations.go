package tmd

// ──────────────────────────────────────────────────────────
// TMD Forecast – Default Target Locations
//
// Major Thai cities / provinces used for pre-caching.
// Extend or override as needed via configuration.
// ──────────────────────────────────────────────────────────

// DefaultLocations returns a curated list of major Thai cities.
func DefaultLocations() []TargetLocation {
	return []TargetLocation{
		{Code: "bangkok", Lat: 13.7563, Lon: 100.5018, Province: "กรุงเทพมหานคร"},
		{Code: "chiang_mai", Lat: 18.7883, Lon: 98.9853, Province: "เชียงใหม่", Amphoe: "เมืองเชียงใหม่"},
		{Code: "chiang_rai", Lat: 19.9105, Lon: 99.8406, Province: "เชียงราย", Amphoe: "เมืองเชียงราย"},
		{Code: "phuket", Lat: 7.8804, Lon: 98.3923, Province: "ภูเก็ต", Amphoe: "เมืองภูเก็ต"},
		{Code: "khon_kaen", Lat: 16.4322, Lon: 102.8236, Province: "ขอนแก่น", Amphoe: "เมืองขอนแก่น"},
		{Code: "nakhon_ratchasima", Lat: 14.9799, Lon: 102.0978, Province: "นครราชสีมา", Amphoe: "เมืองนครราชสีมา"},
		{Code: "hat_yai", Lat: 7.0036, Lon: 100.4747, Province: "สงขลา", Amphoe: "หาดใหญ่"},
		{Code: "udon_thani", Lat: 17.4156, Lon: 102.7872, Province: "อุดรธานี", Amphoe: "เมืองอุดรธานี"},
		{Code: "surat_thani", Lat: 9.1382, Lon: 99.3217, Province: "สุราษฎร์ธานี", Amphoe: "เมืองสุราษฎร์ธานี"},
		{Code: "nonthaburi", Lat: 13.8621, Lon: 100.5144, Province: "นนทบุรี", Amphoe: "เมืองนนทบุรี"},
	}
}
