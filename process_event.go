package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mgagliardo91/go-utils"

	"github.com/mgagliardo91/blacksmith"
	"github.com/mgagliardo91/offline-common"

	geo "github.com/martinlindhe/google-geolocate"
)

type TimeRange struct {
	start int64
	end   int64
}

type DateTime struct {
	date      *time.Time
	timeRange *TimeRange
}

const extractTimeRangeString = `(?P<StartTime>\d{1,2}:\d{2}[ap])(\s+\-\s+(?P<EndTime>\d{1,2}:\d{2}[ap])){0,1}`

var extractPrice = regexp.MustCompile(`(?P<Price>\d+\.*\d+)`)
var extractTimeRange = regexp.MustCompile(extractTimeRangeString)
var extractTimeParts = regexp.MustCompile(`(?P<Hours>\d{1,2}):(?P<Minutes>\d{2})(?P<Period>[ap])`)

var extractDate = regexp.MustCompile(`\w+,\s\w+\s\d{1,2}(st|th|rd)(,\s\d{1,2}:\d{2}[ap]\s+\-\s+\d{1,2}:\d{2}[ap]{0,1}){0,1}`)
var extractDateTime = regexp.MustCompile(`\w+,\s(?P<Month>\w+)\s(?P<Day>\d{1,2})(st|th|rd)(,\s(` + extractTimeRangeString + `)){0,1}`)
var extractDateRange = regexp.MustCompile(`(?P<Start>\w+,\s\w+\s\d{1,2}(st|th|rd))(\s*–\s*){0,1}(?P<End>\w+,\s\w+\s\d{1,2}(st|th|rd))`)

var extractEventID = regexp.MustCompile(`https?:\/\/(www\.)?get-offline\.com\/(?P<EventID>[a-zA-Z0-9]+(\/[a-zA-Z0-9-]+)+)+\??`)

var client = geo.NewGoogleGeo("AIzaSyAXbEiauQpsNQQvxzJyI0nIiVf_JWgTHs4")

func processRawEvent(task blacksmith.Task) {
	rawEvent, ok := task.Payload.(common.RawOfflineEvent)
	if !ok {
		GetLogger().Errorf("Unable to extract OfflineEvent from task %+v", task.Payload)
		return
	}

	event, err := cleanRawEvent(&rawEvent)

	if err != nil {
		GetLogger().Errorf("Unable to process event: %+v", err)
		return
	}

	events := createEventsForEachOccurrence(event, &rawEvent)

	jsonValue, _ := json.Marshal(events)
	GetLogger().Infof("%s", jsonValue)
}

func createEventsForEachOccurrence(event *common.OfflineEvent, rawEvent *common.RawOfflineEvent) (events []*common.OfflineEvent) {
	events = make([]*common.OfflineEvent, 0)
	useFixedTimes := false

	mainStartTime, mainEndTime := cleanTimeRange(getParams(extractTimeRange, rawEvent.TimeRaw))
	if mainStartTime != nil && mainEndTime != nil {
		useFixedTimes = true
	}

	dateTimes, _ := cleanDates(rawEvent)

	for _, dateTime := range dateTimes {
		var start, end time.Time
		eventInstance := common.OfflineEvent(*event)
		GetLogger().Infof("Processing date with epoch value: %d", utils.TimeToMilis(*dateTime.date))

		if useFixedTimes {
			start = dateTime.date.Add(time.Duration(*mainStartTime) * time.Millisecond)
			end = dateTime.date.Add(time.Duration(*mainEndTime) * time.Millisecond)
		} else if dateTime.timeRange != nil {
			start = dateTime.date.Add(time.Duration(dateTime.timeRange.start) * time.Millisecond)
			end = dateTime.date.Add(time.Duration(dateTime.timeRange.end) * time.Millisecond)
		}

		eventInstance.StartDateTime = utils.TimeToMilis(start)
		eventInstance.EndDateTime = utils.TimeToMilis(end)
		generateUniqueID(&eventInstance)
		events = append(events, &eventInstance)
	}

	return
}

func generateUniqueID(event *common.OfflineEvent) {
	event.ID = fmt.Sprintf("%s:%d:%d", event.EventID, event.StartDateTime, event.EndDateTime)
}

func cleanRawEvent(rawEvent *common.RawOfflineEvent) (event *common.OfflineEvent, err error) {
	event = &common.OfflineEvent{
		Description:  rawEvent.Description,
		EventURL:     rawEvent.EventURL,
		ImageURL:     rawEvent.ImageURL,
		ReferralURLs: rawEvent.ReferralURLs,
		Tags:         make([]string, 0),
		Teaser:       rawEvent.Teaser,
		Title:        rawEvent.Title,
	}

	event.Price = cleanPrice(rawEvent)
	err = cleanEventUrl(event, rawEvent.OfflineURL)
	cleanLocation(event, rawEvent.LocationRaw)

	return
}

func cleanEventUrl(event *common.OfflineEvent, url string) error {
	if index := strings.IndexByte(url, '?'); index >= 0 {
		event.OfflineURL = url[:index]
	} else {
		event.OfflineURL = url
	}

	params := getParams(extractEventID, event.OfflineURL)

	eventID, eventIDExists := params["EventID"]

	if !eventIDExists {
		return fmt.Errorf("Unable to extract eventID for url: %s", event.EventURL)
	}

	categoryIndex := strings.IndexByte(eventID, '/')
	event.Category = eventID[:categoryIndex]
	event.EventID = strings.Replace(eventID, "/", "-", -1)

	return nil
}

func cleanLocation(event *common.OfflineEvent, addressRaw string) {
	res, err := client.Geocode(addressRaw)

	if err != nil {
		event.Address = addressRaw
		return
	}

	event.Latitude = res.Lat
	event.Longitude = res.Lng
	event.Address = res.Address
}

func cleanDates(event *common.RawOfflineEvent) ([]DateTime, error) {
	dates := make([]DateTime, 0)

	if extractDateRange.MatchString(event.DateRaw) {
		params := getParams(extractDateRange, event.DateRaw)

		start, startOk := params["Start"]
		end, endOk := params["End"]

		if !startOk || !endOk {
			return nil, fmt.Errorf("Cannot parse date range: %s", event.DateRaw)
		}

		startDateTime, err := cleanDateTime(getParams(extractDateTime, strings.TrimSpace(start)))

		if err != nil {
			return nil, err
		}

		endDateTime, err := cleanDateTime(getParams(extractDateTime, strings.TrimSpace(end)))

		if err != nil {
			return nil, err
		}

		err = forEachDay(startDateTime, endDateTime, func(date *DateTime) {
			dates = append(dates, *date)
		})

		return dates, err
	} else if extractDate.MatchString(event.DateRaw) {
		allParams := getAllParams(extractDateTime, event.DateRaw)

		for _, match := range allParams {
			dateTime, err := cleanDateTime(match)

			if err != nil {
				return nil, err
			}

			dates = append(dates, *dateTime)
		}

		return dates, nil
	}

	return nil, errors.New("Nothing to do")
}

type DateConsumer func(*DateTime)

func forEachDay(start *DateTime, end *DateTime, consumer DateConsumer) error {
	if start.date.After(*end.date) {
		return fmt.Errorf("StartDate cannot be after EndDate. Start=%+v End=%+v", start.date, end.date)
	}

	nextDay := *start.date

	for (nextDay).Before(*end.date) {
		nextDayValue := time.Time(nextDay)
		consumer(&DateTime{
			date:      &nextDayValue,
			timeRange: start.timeRange,
		})

		nextDay = nextDay.AddDate(0, 0, 1)
	}

	return nil
}

func cleanDateTime(params map[string]string) (*DateTime, error) {
	month, monthOk := params["Month"]
	day, dayOk := params["Day"]

	if !monthOk || !dayOk {
		return nil, fmt.Errorf("Unable to extract month and day")
	}

	EST, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(EST)
	date, err := time.ParseInLocation("2006 January 2", fmt.Sprintf("%d %s %s", now.Year(), month, day), EST)

	if err != nil {
		return nil, fmt.Errorf("Unable to create date. Error: %+v", err)
	}

	startTimePtr, endTimePtr := cleanTimeRange(params)
	var timeRange *TimeRange
	if startTimePtr != nil && endTimePtr != nil {
		timeRange = &TimeRange{start: *startTimePtr, end: *endTimePtr}
	}

	return &DateTime{date: &date, timeRange: timeRange}, nil
}

func cleanTimeRange(params map[string]string) (startTimePtr *int64, endTimePtr *int64) {
	if startTime, ok := params["StartTime"]; ok {
		startTimePtr, _ = convertTimeExpressionToMilis(startTime)
	}

	if endTime, ok := params["EndTime"]; ok {
		var err error
		if endTimePtr, err = convertTimeExpressionToMilis(endTime); err != nil {
			endTimePtr = &(*startTimePtr)
		}
	}

	return
}

func convertTimeExpressionToMilis(value string) (*int64, error) {
	params := getParams(extractTimeParts, value)

	hours, hoursOk := params["Hours"]
	minutes, minutesOk := params["Minutes"]
	period, periodOk := params["Period"]

	if !hoursOk || !minutesOk || !periodOk {
		return nil, fmt.Errorf("Invalid time expression for value: %s", value)
	}

	numericHours, err := strconv.ParseInt(hours, 10, 64)
	if err != nil {
		return nil, err
	}

	numericMinutes, err := strconv.ParseInt(minutes, 10, 64)
	if err != nil {
		return nil, err
	}

	milis := int64(time.Duration(numericHours)*time.Hour/time.Millisecond) + int64(time.Duration(numericMinutes)*time.Minute/time.Millisecond)

	if period == "p" {
		milis += int64(12 * time.Hour / time.Millisecond)
	}

	return &milis, nil
}

func cleanPrice(event *common.RawOfflineEvent) float64 {
	if event.PriceRaw == "" || strings.ToLower(event.PriceRaw) == "free" {
		return 0.0
	}

	match := extractPrice.FindStringSubmatch(event.PriceRaw)
	if len(match) > 0 {
		valString := match[0]

		if val, err := strconv.ParseFloat(valString, 32); err == nil {
			return val
		}
	}

	return 0.0
}

func getParams(regEx *regexp.Regexp, value string) (paramsMap map[string]string) {
	params := getAllParams(regEx, value)

	if len(params) > 0 {
		return params[0]
	}

	return make(map[string]string)
}

func getAllParams(regEx *regexp.Regexp, value string) (paramsMap []map[string]string) {
	allMatches := regEx.FindAllStringSubmatch(value, -1)

	paramsMap = make([]map[string]string, len(allMatches))

	for j, match := range allMatches {
		paramsMap[j] = make(map[string]string)

		for i, name := range regEx.SubexpNames() {
			if i > 0 && i <= len(match) && len(match[i]) > 0 {
				paramsMap[j][name] = match[i]
			}
		}
	}
	return
}
