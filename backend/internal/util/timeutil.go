package util

import "time"

const BeijingTimezone = "Asia/Shanghai"

var beijingLocation = mustLoadBeijingLocation()

func mustLoadBeijingLocation() *time.Location {
	loc, err := time.LoadLocation(BeijingTimezone)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func BeijingLocation() *time.Location {
	return beijingLocation
}

func ForceBeijingTimezone() {
	time.Local = beijingLocation
}

func NowBeijing() time.Time {
	return time.Now().In(beijingLocation)
}

func BeijingDayRange(now time.Time) (time.Time, time.Time) {
	local := now.In(beijingLocation)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, beijingLocation)
	return start, start.Add(24 * time.Hour)
}
