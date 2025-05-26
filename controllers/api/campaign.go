package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Helper function to process events into timeline data
func processEventsForTimeline(events []models.Event) map[string][][2]int64 {
	timeline := make(map[string][][2]int64)
	// Temporary map to aggregate counts by day and message
	// map[message][day_timestamp_sec]count
	aggregated := make(map[string]map[int64]int64)

	for _, event := range events {
		// Normalize time to the beginning of the day (UTC)
		dayTimestamp := time.Date(event.Time.Year(), event.Time.Month(), event.Time.Day(), 0, 0, 0, 0, time.UTC).Unix()

		if _, ok := aggregated[event.Message]; !ok {
			aggregated[event.Message] = make(map[int64]int64)
		}
		aggregated[event.Message][dayTimestamp]++
	}

	// Convert aggregated data to the desired slice format and sort by timestamp
	for message, dailyCounts := range aggregated {
		var points [][2]int64
		for dayTimestamp, count := range dailyCounts {
			points = append(points, [2]int64{dayTimestamp * 1000, count}) // Convert to milliseconds
		}
		// Sort points by timestamp
		sort.Slice(points, func(i, j int) bool {
			return points[i][0] < points[j][0]
		})
		timeline[message] = points
	}
	return timeline
}

// Helper function to process events for User-Agent distribution
func processEventsForUserAgentDistribution(events []models.Event) map[string]int {
	uaCounts := make(map[string]int)
	relevantEventTypes := map[string]bool{
		models.EventOpened:      true,
		models.EventClicked:     true,
		models.EventDataSubmit:  true,
	}

	for _, event := range events {
		if _, ok := relevantEventTypes[event.Message]; !ok {
			continue // Skip events not relevant for UA tracking
		}

		if event.Details == "" {
			continue // Skip if no details are present
		}

		var details models.EventDetails
		err := json.Unmarshal([]byte(event.Details), &details)
		if err != nil {
			log.Warnf("Error unmarshalling event details for event ID %d (campaign %d): %v", event.Id, event.CampaignId, err)
			continue // Skip if details can't be parsed
		}

		if details.Browser == nil {
			continue // Skip if browser details are missing
		}

		userAgent, ok := details.Browser["User-Agent"]
		if !ok || userAgent == "" {
			userAgent = "Unknown" // Group empty or missing User-Agents
		}
		uaCounts[userAgent]++
	}
	return uaCounts
}

// Helper function to process results for IP address distribution
func processResultsForIPDistribution(results []models.Result) map[string]int {
	ipCounts := make(map[string]int)
	for _, result := range results {
		ip := strings.TrimSpace(result.IP)
		if ip == "" {
			ip = "Unknown"
		}
		ipCounts[ip]++
	}
	return ipCounts
}

// Helper function to process events for hourly distribution
func processEventsForHourlyDistribution(events []models.Event) map[string]int {
	hourlyCounts := make(map[string]int)
	for i := 0; i < 24; i++ {
		hourlyCounts[strconv.Itoa(i)] = 0 // Initialize all hours to 0
	}

	relevantEventTypes := map[string]bool{
		models.EventOpened:     true,
		models.EventClicked:    true,
		models.EventDataSubmit: true,
	}

	for _, event := range events {
		if _, ok := relevantEventTypes[event.Message]; !ok {
			continue // Skip events not relevant for this distribution
		}
		hour := event.Time.UTC().Hour() // Get hour in UTC
		hourlyCounts[strconv.Itoa(hour)]++
	}
	return hourlyCounts
}

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}

// GetAggregatedCampaignSummary returns the aggregated statistics for a list of campaign IDs.
func (as *Server) GetAggregatedCampaignSummary(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	if user.Id == 0 {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusUnauthorized)}, http.StatusUnauthorized)
		return
	}

	campaignIDsStr := r.URL.Query().Get("campaign_ids")
	if campaignIDsStr == "" {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids parameter is required"}, http.StatusBadRequest)
		return
	}

	idsStr := strings.Split(campaignIDsStr, ",")
	var campaignIDs []int64
	for _, idStr := range idsStr {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_id: " + idStr}, http.StatusBadRequest)
			return
		}
		campaignIDs = append(campaignIDs, id)
	}

	if len(campaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaign_ids provided"}, http.StatusBadRequest)
		return
	}

	stats, err := models.GetAggregatedCampaignStats(campaignIDs, user.Id)
	if err != nil {
		// Check for specific errors like gorm.ErrRecordNotFound if GetAggregatedCampaignStats returns them
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "One or more campaigns not found or not accessible"}, http.StatusNotFound)
		} else {
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching aggregated campaign stats: " + err.Error()}, http.StatusInternalServerError)
		}
		log.Error(err)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// GetCampaignTimeline returns the timeline data for a list of campaign IDs.
func (as *Server) GetCampaignTimeline(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	if user.Id == 0 {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusUnauthorized)}, http.StatusUnauthorized)
		return
	}

	campaignIDsStr := r.URL.Query().Get("campaign_ids")
	if campaignIDsStr == "" {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids parameter is required"}, http.StatusBadRequest)
		return
	}

	idsStr := strings.Split(campaignIDsStr, ",")
	var campaignIDs []int64
	for _, idStr := range idsStr {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_id: " + idStr}, http.StatusBadRequest)
			return
		}
		campaignIDs = append(campaignIDs, id)
	}

	if len(campaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaign_ids provided"}, http.StatusBadRequest)
		return
	}

	events, err := models.GetEventsByCampaignIDs(campaignIDs, user.Id)
	if err != nil {
		// Check for specific errors like gorm.ErrRecordNotFound if GetEventsByCampaignIDs returns them
		if err == gorm.ErrRecordNotFound { // This might not be directly returned by GetEventsByCampaignIDs itself but underlying calls
			JSONResponse(w, models.Response{Success: false, Message: "No events found for the given campaigns or campaigns not accessible"}, http.StatusNotFound)
		} else {
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching campaign events: " + err.Error()}, http.StatusInternalServerError)
		}
		log.Error(err)
		return
	}

	timelineData := processEventsForTimeline(events)
	JSONResponse(w, timelineData, http.StatusOK)
}

// GetCampaignUserAgentDistribution returns the user-agent distribution for a list of campaign IDs.
func (as *Server) GetCampaignUserAgentDistribution(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	if user.Id == 0 {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusUnauthorized)}, http.StatusUnauthorized)
		return
	}

	campaignIDsStr := r.URL.Query().Get("campaign_ids")
	if campaignIDsStr == "" {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids parameter is required"}, http.StatusBadRequest)
		return
	}

	idsStr := strings.Split(campaignIDsStr, ",")
	var campaignIDs []int64
	for _, idStr := range idsStr {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_id: " + idStr}, http.StatusBadRequest)
			return
		}
		campaignIDs = append(campaignIDs, id)
	}

	if len(campaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaign_ids provided"}, http.StatusBadRequest)
		return
	}

	events, err := models.GetEventsByCampaignIDs(campaignIDs, user.Id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "No events found for the given campaigns or campaigns not accessible"}, http.StatusNotFound)
		} else {
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching campaign events: " + err.Error()}, http.StatusInternalServerError)
		}
		log.Error(err)
		return
	}

	userAgentData := processEventsForUserAgentDistribution(events)
	JSONResponse(w, userAgentData, http.StatusOK)
}

// GetCampaignIPDistribution returns the IP address distribution for a list of campaign IDs.
func (as *Server) GetCampaignIPDistribution(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	if user.Id == 0 {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusUnauthorized)}, http.StatusUnauthorized)
		return
	}

	campaignIDsStr := r.URL.Query().Get("campaign_ids")
	if campaignIDsStr == "" {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids parameter is required"}, http.StatusBadRequest)
		return
	}

	idsStr := strings.Split(campaignIDsStr, ",")
	var campaignIDs []int64
	for _, idStr := range idsStr {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_id: " + idStr}, http.StatusBadRequest)
			return
		}
		campaignIDs = append(campaignIDs, id)
	}

	if len(campaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaign_ids provided"}, http.StatusBadRequest)
		return
	}

	relevantStatuses := []string{
		models.EventOpened,
		models.EventClicked,
		models.EventDataSubmit,
	}

	results, err := models.GetResultsByCampaignIDsAndStatuses(campaignIDs, relevantStatuses, user.Id)
	if err != nil {
		// Potentially check for gorm.ErrRecordNotFound if that's a possible distinct error from GetResultsByCampaignIDsAndStatuses
		JSONResponse(w, models.Response{Success: false, Message: "Error fetching results for IP distribution: " + err.Error()}, http.StatusInternalServerError)
		log.Error(err)
		return
	}

	ipDistributionData := processResultsForIPDistribution(results)
	JSONResponse(w, ipDistributionData, http.StatusOK)
}

// GetCampaignHourlyDistribution returns the hourly event distribution for a list of campaign IDs.
func (as *Server) GetCampaignHourlyDistribution(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	if user.Id == 0 {
		JSONResponse(w, models.Response{Success: false, Message: http.StatusText(http.StatusUnauthorized)}, http.StatusUnauthorized)
		return
	}

	campaignIDsStr := r.URL.Query().Get("campaign_ids")
	if campaignIDsStr == "" {
		JSONResponse(w, models.Response{Success: false, Message: "campaign_ids parameter is required"}, http.StatusBadRequest)
		return
	}

	idsStr := strings.Split(campaignIDsStr, ",")
	var campaignIDs []int64
	for _, idStr := range idsStr {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_id: " + idStr}, http.StatusBadRequest)
			return
		}
		campaignIDs = append(campaignIDs, id)
	}

	if len(campaignIDs) == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "No campaign_ids provided"}, http.StatusBadRequest)
		return
	}

	events, err := models.GetEventsByCampaignIDs(campaignIDs, user.Id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "No events found for the given campaigns or campaigns not accessible"}, http.StatusNotFound)
		} else {
			JSONResponse(w, models.Response{Success: false, Message: "Error fetching campaign events: " + err.Error()}, http.StatusInternalServerError)
		}
		log.Error(err)
		return
	}

	hourlyData := processEventsForHourlyDistribution(events)
	JSONResponse(w, hourlyData, http.StatusOK)
}
