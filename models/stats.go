package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/mssola/user_agent" // Assuming this can be added
	log "github.com/gophish/gophish/logger"
)

// UserAgentStats holds aggregated data about user agents.
type UserAgentStats struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Count   int    `json:"count"`
}

// IPStats holds aggregated data about IP addresses.
type IPStats struct {
	IPAddress string `json:"ip_address"`
	Count     int    `json:"count"`
}

// TimelinePoint represents a single data point in a time series chart.
type TimelinePoint struct {
	Time  string `json:"time"` // Could be date string or hour string
	Count int    `json:"count"`
}

// WorkHourDistribution holds the count of events during work hours vs. off hours.
// Using a map for flexibility, but a struct could also be used.
type WorkHourDistribution map[string]int

// GetCombinedCampaignStats calculates the sum of stats for multiple campaigns.
// It reuses the existing CampaignStats struct.
func GetCombinedCampaignStats(campaignIDs []int64) (CampaignStats, error) {
	combinedStats := CampaignStats{}
	if len(campaignIDs) == 0 {
		return combinedStats, nil
	}

	// Ensure we only query for campaigns the user has access to,
	// though this function itself doesn't know about the user.
	// The calling function (API handler) should enforce user access rights.
	
	// In GORM, `db.Where("id IN (?)", ids)` is used for querying multiple IDs.
	// We need to fetch individual stats for each campaign and aggregate them.
	// The `getCampaignStats` function (from campaign.go) is for a single campaign.

	for _, id := range campaignIDs {
		// This assumes that `getCampaignStats` is accessible and correctly fetches stats
		// for a single campaign ID. If `getCampaignStats` handles permissions or needs
		// a user ID, this approach might need adjustment or the calling function
		// must ensure `campaignIDs` are already filtered for user access.
		// For now, proceeding with the assumption that `getCampaignStats` is usable as is.
		stats, err := getCampaignStats(id) // This function is in campaign.go
		if err != nil {
			// Decide on error handling: return on first error, or try to aggregate what we can?
			// For now, return on first error to be safe.
			log.Errorf("Error getting stats for campaign ID %d: %v", id, err)
			return CampaignStats{}, fmt.Errorf("error getting stats for campaign ID %d: %w", id, err)
		}
		combinedStats.Total += stats.Total
		combinedStats.EmailsSent += stats.EmailsSent
		combinedStats.OpenedEmail += stats.OpenedEmail
		combinedStats.ClickedLink += stats.ClickedLink
		combinedStats.SubmittedData += stats.SubmittedData
		combinedStats.EmailReported += stats.EmailReported
		combinedStats.Error += stats.Error
	}

	return combinedStats, nil
}

// GetUserAgentStats aggregates user agent data for specified campaigns and event type.
func GetUserAgentStats(campaignIDs []int64, eventType string) ([]UserAgentStats, error) {
	if len(campaignIDs) == 0 {
		return []UserAgentStats{}, nil
	}

	var events []Event
	err := db.Where("campaign_id IN (?) AND message = ?", campaignIDs, eventType).Find(&events).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []UserAgentStats{}, nil // No events found is not an error
		}
		log.Errorf("Error fetching events for UA stats (campaigns: %v, event: %s): %v", campaignIDs, eventType, err)
		return nil, fmt.Errorf("error fetching events: %w", err)
	}

	statsMap := make(map[string]map[string]int) // OS -> Browser -> Count

	for _, event := range events {
		if event.Details == "" {
			continue
		}
		var details EventDetails
		if err := json.Unmarshal([]byte(event.Details), &details); err != nil {
			log.Warnf("Error unmarshalling event details for event ID %d: %v. Details: %s", event.Id, err, event.Details)
			continue
		}

		uaString, ok := details.Browser["user-agent"]
		if !ok || uaString == "" {
			continue
		}

		ua := user_agent.New(uaString)
		osName := ua.OSInfo().Name
		browserName, _ := ua.Browser()

		if osName == "" {
			osName = "Unknown"
		}
		if browserName == "" {
			browserName = "Unknown"
		}

		if _, ok := statsMap[osName]; !ok {
			statsMap[osName] = make(map[string]int)
		}
		statsMap[osName][browserName]++
	}

	var result []UserAgentStats
	for os, browserMap := range statsMap {
		for browser, count := range browserMap {
			result = append(result, UserAgentStats{OS: os, Browser: browser, Count: count})
		}
	}
	return result, nil
}

// GetIPStats aggregates IP address data for specified campaigns and event type.
func GetIPStats(campaignIDs []int64, eventType string) ([]IPStats, error) {
	if len(campaignIDs) == 0 {
		return []IPStats{}, nil
	}

	var events []Event
	// Assuming IP information is in Event.Details.Browser["address"]
	// If it's directly in Result.IP, the query needs to be on the results table
	// and potentially join with events if eventType filtering is strict on event messages.
	// For now, let's assume events table and details JSON.
	err := db.Where("campaign_id IN (?) AND message = ?", campaignIDs, eventType).Find(&events).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []IPStats{}, nil
		}
		log.Errorf("Error fetching events for IP stats (campaigns: %v, event: %s): %v", campaignIDs, eventType, err)
		return nil, fmt.Errorf("error fetching events: %w", err)
	}

	statsMap := make(map[string]int) // IPAddress -> Count

	for _, event := range events {
		if event.Details == "" {
			continue
		}
		var details EventDetails
		if err := json.Unmarshal([]byte(event.Details), &details); err != nil {
			log.Warnf("Error unmarshalling event details for event ID %d: %v. Details: %s", event.Id, err, event.Details)
			continue
		}

		ipAddress, ok := details.Browser["address"] // Assuming "address" key holds the IP
		if !ok || ipAddress == "" {
			// Fallback: check if the Result model might have this info more directly if this event is tied to a result.
			// This would require a different query strategy (querying Results table).
			// For now, sticking to Event details.
			continue
		}
		
		// Basic validation/cleanup for IP if needed
		ipAddress = strings.TrimSpace(ipAddress)
		if ipAddress == "" {
			continue
		}

		statsMap[ipAddress]++
	}

	var result []IPStats
	for ip, count := range statsMap {
		result = append(result, IPStats{IPAddress: ip, Count: count})
	}
	return result, nil
}

// GetEventTimeline aggregates event counts over time.
func GetEventTimeline(campaignIDs []int64, eventType string, period string) ([]TimelinePoint, error) {
	if len(campaignIDs) == 0 {
		return []TimelinePoint{}, nil
	}

	var events []Event
	err := db.Where("campaign_id IN (?) AND message = ?", campaignIDs, eventType).Order("time asc").Find(&events).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []TimelinePoint{}, nil
		}
		log.Errorf("Error fetching events for timeline (campaigns: %v, event: %s): %v", campaignIDs, eventType, err)
		return nil, fmt.Errorf("error fetching events: %w", err)
	}

	if len(events) == 0 {
		return []TimelinePoint{}, nil
	}
	
	layout := ""
	switch strings.ToLower(period) {
	case "hourly":
		layout = "2006-01-02 15:00" // Groups by hour
	case "daily":
		layout = "2006-01-02" // Groups by day
	case "monthly":
		layout = "2006-01" // Groups by month
	default:
		log.Warnf("Invalid period specified for timeline: %s. Defaulting to daily.", period)
		layout = "2006-01-02"
		period = "daily"
	}

	statsMap := make(map[string]int) // TimeKey (formatted) -> Count
	var orderedTimeKeys []string // To maintain order

	for _, event := range events {
		// Ensure event time is in UTC for consistent grouping, though AddEvent already stores in UTC.
		timeKey := event.Time.UTC().Format(layout)
		if _, exists := statsMap[timeKey]; !exists {
			orderedTimeKeys = append(orderedTimeKeys, timeKey)
		}
		statsMap[timeKey]++
	}
	
	// Fill gaps if necessary, especially for hourly/daily over a range.
	// For simplicity, this version only returns points where events occurred.
	// A more complete version would iterate from the first event's period to the last,
	// filling in zero counts for periods with no events.

	var result []TimelinePoint
	for _, key := range orderedTimeKeys {
		result = append(result, TimelinePoint{Time: key, Count: statsMap[key]})
	}

	return result, nil
}

// GetWorkHourDistribution classifies events into work hours vs. off-hours.
func GetWorkHourDistribution(campaignIDs []int64, eventType string) (WorkHourDistribution, error) {
	if len(campaignIDs) == 0 {
		return WorkHourDistribution{}, nil
	}
	
	distribution := WorkHourDistribution{
		"Work Hours": 0,
		"Off Hours":  0,
	}

	var events []Event
	err := db.Where("campaign_id IN (?) AND message = ?", campaignIDs, eventType).Find(&events).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return distribution, nil
		}
		log.Errorf("Error fetching events for work hour distribution (campaigns: %v, event: %s): %v", campaignIDs, eventType, err)
		return nil, fmt.Errorf("error fetching events: %w", err)
	}

	for _, event := range events {
		// Assuming event.Time is already in UTC as stored by AddEvent
		eventTime := event.Time 
		weekday := eventTime.Weekday()
		hour := eventTime.Hour()

		// Work Hours: Monday-Friday, 8 AM (8) to 7 PM (19) (exclusive of 19:00)
		if weekday >= time.Monday && weekday <= time.Friday && hour >= 8 && hour < 19 {
			distribution["Work Hours"]++
		} else {
			distribution["Off Hours"]++
		}
	}
	return distribution, nil
}
