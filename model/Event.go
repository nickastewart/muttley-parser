package model 

type Event struct {
	Date		string
	Location    string
	RaceType    string
	DriverInfo  DriverInfo
	DriverTimes []DriverTime
}
