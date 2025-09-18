package model 

type Event struct {
	Date		string
	Location    string
	RaceType    string
	Position    string
	DriverInfo  DriverInfo
	DriverTimes []DriverTime
}
