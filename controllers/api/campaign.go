package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/o365auth" // Keep if PollO365Token is still in use elsewhere or future
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, ErrorResponse{Message: "Error fetching campaigns", Details: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, ErrorResponse{Message: "Invalid JSON structure", Details: err.Error()}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, ErrorResponse{Message: err.Error()}, http.StatusBadRequest)
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
			JSONResponse(w, ErrorResponse{Message: "Error fetching campaign summaries", Details: err.Error()}, http.StatusInternalServerError)
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
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, ErrorResponse{Message: "Campaign not found"}, http.StatusNotFound)
		} else {
			log.Error(err)
			JSONResponse(w, ErrorResponse{Message: "Error fetching campaign", Details: err.Error()}, http.StatusInternalServerError)
		}
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id) // Authorization for deletion is implicitly handled by GetCampaign ensuring user ownership.
		if err != nil {
			log.Error(err)
			JSONResponse(w, ErrorResponse{Message: "Error deleting campaign", Details: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, SuccessResponse{Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	// First, ensure the user has access to this campaign
	_, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, ErrorResponse{Message: "Campaign not found or insufficient permissions"}, http.StatusNotFound)
		} else {
			log.Error(err)
			JSONResponse(w, ErrorResponse{Message: "Error verifying campaign access", Details: err.Error()}, http.StatusInternalServerError)
		}
		return
	}
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, ErrorResponse{Message: "Error fetching campaign results", Details: err.Error()}, http.StatusInternalServerError)
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
				JSONResponse(w, ErrorResponse{Message: "Campaign not found or insufficient permissions"}, http.StatusNotFound)
			} else {
				log.Error(err)
				JSONResponse(w, ErrorResponse{Message: "Error fetching campaign summary", Details: err.Error()}, http.StatusInternalServerError)
			}
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
	case r.Method == "POST":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, ErrorResponse{Message: "Campaign not found or insufficient permissions"}, http.StatusNotFound)
			} else {
				log.Error(err)
				JSONResponse(w, ErrorResponse{Message: "Error completing campaign", Details: err.Error()}, http.StatusInternalServerError)
			}
			return
		}
		JSONResponse(w, SuccessResponse{Message: "Campaign completed successfully!"}, http.StatusOK)
	default:
		JSONResponse(w, ErrorResponse{Message: "Method not allowed. Use POST."}, http.StatusMethodNotAllowed)
	}
}

// PollO365Token handles requests to poll for an O365 token for a specific recipient.
// POST /api/campaigns/{id}/results/{rid}/poll_token
func (as *Server) PollO365Token(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	campaignID, err := parseInt64(vars["id"])
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Invalid campaign ID", Details: err.Error()}, http.StatusBadRequest)
		return
	}
	rid := vars["rid"]
	if rid == "" {
		JSONResponse(w, ErrorResponse{Message: "Result ID (rid) not specified in path"}, http.StatusBadRequest)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	campaign, err := models.GetCampaign(campaignID, uid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, ErrorResponse{Message: "Campaign not found or insufficient permissions"}, http.StatusNotFound)
		} else {
			log.Errorf("Error getting campaign %d for user %d: %v", campaignID, uid, err)
			JSONResponse(w, ErrorResponse{Message: "Error retrieving campaign", Details: err.Error()}, http.StatusInternalServerError)
		}
		return
	}

	if campaign.Type != models.CampaignDeviceToken {
		JSONResponse(w, ErrorResponse{Message: "This endpoint is only for Device Token type campaigns"}, http.StatusBadRequest)
		return
	}

	result, err := models.GetResult(rid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, ErrorResponse{Message: "Result not found"}, http.StatusNotFound)
		} else {
			log.Errorf("Error getting result %s for campaign %d: %v", rid, campaignID, err)
			JSONResponse(w, ErrorResponse{Message: "Error retrieving result", Details: err.Error()}, http.StatusInternalServerError)
		}
		return
	}

	if result.CampaignId != campaign.Id {
		JSONResponse(w, ErrorResponse{Message: "Result does not belong to the specified campaign"}, http.StatusBadRequest)
		return
	}

	if result.UserId != uid {
		log.Warnf("User %d attempted to poll token for result %s belonging to user %d via campaign %d", uid, rid, result.UserId, campaignID)
		JSONResponse(w, ErrorResponse{Message: "Result ownership mismatch"}, http.StatusForbidden)
		return
	}

	if result.DeviceCode == "" {
		JSONResponse(w, ErrorResponse{Message: "Device code not initiated for this recipient"}, http.StatusBadRequest)
		return
	}

	if result.AccessToken != "" && result.TokenExpiry.After(time.Now().UTC()) {
		JSONResponse(w, Response{Message: "Tokens already available and valid", Status: "success"}, http.StatusOK)
		return
	}

	if result.DeviceCodeExpiry.Before(time.Now().UTC()) {
		errMsg := "Device code has expired. Please restart the authentication process on the landing page."
		if result.Status != "Device Code Expired" {
			result.Status = "Device Code Expired"
			if errDb := models.PutResult(&result); errDb != nil {
				log.Errorf("Error saving expired status for result %s: %v", result.RId, errDb)
			}
			models.AddEvent(&models.Event{CampaignId: campaign.Id, Email: result.Email, Message: "Device Code Expired", Details: "Attempted to poll with expired device code."}, campaign.Id)
		}
		JSONResponse(w, ErrorResponse{Message: errMsg, Status: "expired_token"}, http.StatusBadRequest)
		return
	}

	const clientID = "YOUR_CLIENT_ID_HERE" // This should be configurable

	tokenResponse, tokenStatus, errAuth := o365auth.CheckTokenStatusOnce(clientID, result.DeviceCode)
	var eventDetailsString string

	switch tokenStatus {
	case "success":
		result.AccessToken = tokenResponse.AccessToken
		result.RefreshToken = tokenResponse.RefreshToken
		result.TokenExpiry = time.Now().UTC().Add(time.Second * time.Duration(tokenResponse.ExpiresIn))
		previousStatus := result.Status
		result.Status = models.EventDeviceTokenObtained
		if errDb := models.PutResult(&result); errDb != nil {
			log.Errorf("Error saving result with token for rid %s: %v", result.RId, errDb)
			JSONResponse(w, ErrorResponse{Message: "Failed to save token", Status: "error", Details: errDb.Error()}, http.StatusInternalServerError)
			return
		}
		if previousStatus != models.EventDeviceTokenObtained {
			eventDetailsString = fmt.Sprintf("Access token obtained for %s", result.Email)
			if result.Email == "" {
				eventDetailsString = "Access token obtained"
			}
			models.AddEvent(&models.Event{CampaignId: campaign.Id, Email: result.Email, Message: models.EventDeviceTokenObtained, Details: eventDetailsString}, campaign.Id)
		}
		JSONResponse(w, Response{Message: "Tokens obtained successfully", Status: "success"}, http.StatusOK)
	case "authorization_pending":
		JSONResponse(w, Response{Message: "User authorization still pending", Status: "pending"}, http.StatusOK)
	case "expired_token", "authorization_declined", "bad_verification_code":
		statusMessage := "O365 Token Error: " + tokenStatus
		if errAuth != nil {
			eventDetailsString = errAuth.Error()
			statusMessage = fmt.Sprintf("O365 Token Error: %s - %s", tokenStatus, errAuth.Error())
		} else {
			eventDetailsString = tokenStatus
		}
		var newResultStatus string
		var eventMessage string
		switch tokenStatus {
		case "expired_token":
			newResultStatus = "Device Code Expired"
			eventMessage = "Device Code Expired"
		case "authorization_declined":
			newResultStatus = "Authorization Declined"
			eventMessage = "Authorization Declined"
		case "bad_verification_code":
			newResultStatus = "Bad Verification Code"
			eventMessage = "Bad Verification Code"
		}
		if result.Status != newResultStatus {
			result.Status = newResultStatus
			if errDb := models.PutResult(&result); errDb != nil {
				log.Errorf("Error saving %s status for result %s: %v", newResultStatus, result.RId, errDb)
			}
			models.AddEvent(&models.Event{CampaignId: campaign.Id, Email: result.Email, Message: eventMessage, Details: eventDetailsString}, campaign.Id)
		}
		JSONResponse(w, ErrorResponse{Message: statusMessage, Status: tokenStatus}, http.StatusBadRequest)
	case "communication_error", "unknown_error":
		log.Errorf("Error polling token status for rid %s (device_code %s): %v (status: %s)", result.RId, result.DeviceCode, errAuth, tokenStatus)
		errMsg := "Error polling token status: " + tokenStatus
		if errAuth != nil {
			errMsg = fmt.Sprintf("Error polling token status: %s - %s", tokenStatus, errAuth.Error())
		}
		JSONResponse(w, ErrorResponse{Message: errMsg, Status: "error"}, http.StatusInternalServerError)
	default:
		log.Errorf("Unhandled token status for rid %s: %s. Error: %v", result.RId, tokenStatus, errAuth)
		errMsgDefault := "Unknown token status"
		if errAuth != nil {
			errMsgDefault = fmt.Sprintf("Unknown token status: %s", errAuth.Error())
		}
		JSONResponse(w, ErrorResponse{Message: errMsgDefault, Status: "error"}, http.StatusInternalServerError)
	}
}

// Helper function to parse comma-separated IDs and check authorization
func (as *Server) parseAndAuthorizeCampaignIDs(r *http.Request, paramName string) ([]int64, error) {
	idsStr := r.URL.Query().Get(paramName)
	if idsStr == "" {
		return nil, fmt.Errorf("%s parameter is required", paramName)
	}

	idStrs := strings.Split(idsStr, ",")
	parsedIDs := make([]int64, 0, len(idStrs))
	userID := ctx.Get(r, "user_id").(int64)

	for _, idStr := range idStrs {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid campaign ID: %s", idStr)
		}
		// Authorization check: Ensure the user has access to this campaign
		_, err = models.GetCampaign(id, userID) // This function checks ownership/permissions
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("campaign with ID %d not found or access denied", id)
			}
			log.Errorf("Error checking access for campaign ID %d: %v", id, err)
			return nil, fmt.Errorf("error verifying access to campaign ID %d", id)
		}
		parsedIDs = append(parsedIDs, id)
	}
	if len(parsedIDs) == 0 {
		return nil, fmt.Errorf("no valid campaign IDs provided or authorized")
	}
	return parsedIDs, nil
}

// GetCampaignStatsCombined handles requests for combined campaign statistics.
// GET /api/stats/campaigns/combined?campaign_ids=1,2,3
func (as *Server) GetCampaignStatsCombined(w http.ResponseWriter, r *http.Request) {
	campaignIDs, err := as.parseAndAuthorizeCampaignIDs(r, "campaign_ids")
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Failed to parse or authorize campaign IDs", Details: err.Error()}, http.StatusBadRequest)
		return
	}

	stats, err := models.GetCombinedCampaignStats(campaignIDs)
	if err != nil {
		log.Errorf("Error getting combined campaign stats: %v", err)
		JSONResponse(w, ErrorResponse{Message: "Error calculating combined campaign stats", Details: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// GetUserAgentStatsReport handles requests for user agent statistics.
// GET /api/stats/user_agent?campaign_ids=1,2,3&event_type=Clicked Link
func (as *Server) GetUserAgentStatsReport(w http.ResponseWriter, r *http.Request) {
	campaignIDs, err := as.parseAndAuthorizeCampaignIDs(r, "campaign_ids")
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Failed to parse or authorize campaign IDs", Details: err.Error()}, http.StatusBadRequest)
		return
	}
	eventType := r.URL.Query().Get("event_type")
	if eventType == "" {
		JSONResponse(w, ErrorResponse{Message: "event_type parameter is required"}, http.StatusBadRequest)
		return
	}

	stats, err := models.GetUserAgentStats(campaignIDs, eventType)
	if err != nil {
		log.Errorf("Error getting user agent stats: %v", err)
		JSONResponse(w, ErrorResponse{Message: "Error generating user agent stats", Details: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// GetIPStatsReport handles requests for IP address statistics.
// GET /api/stats/ip?campaign_ids=1,2,3&event_type=Clicked Link
func (as *Server) GetIPStatsReport(w http.ResponseWriter, r *http.Request) {
	campaignIDs, err := as.parseAndAuthorizeCampaignIDs(r, "campaign_ids")
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Failed to parse or authorize campaign IDs", Details: err.Error()}, http.StatusBadRequest)
		return
	}
	eventType := r.URL.Query().Get("event_type")
	if eventType == "" {
		JSONResponse(w, ErrorResponse{Message: "event_type parameter is required"}, http.StatusBadRequest)
		return
	}

	stats, err := models.GetIPStats(campaignIDs, eventType)
	if err != nil {
		log.Errorf("Error getting IP stats: %v", err)
		JSONResponse(w, ErrorResponse{Message: "Error generating IP stats", Details: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// GetEventTimelineReport handles requests for event timeline statistics.
// GET /api/stats/timeline?campaign_ids=1,2,3&event_type=Clicked Link&period=daily
func (as *Server) GetEventTimelineReport(w http.ResponseWriter, r *http.Request) {
	campaignIDs, err := as.parseAndAuthorizeCampaignIDs(r, "campaign_ids")
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Failed to parse or authorize campaign IDs", Details: err.Error()}, http.StatusBadRequest)
		return
	}
	eventType := r.URL.Query().Get("event_type")
	if eventType == "" {
		JSONResponse(w, ErrorResponse{Message: "event_type parameter is required"}, http.StatusBadRequest)
		return
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily" // Default period
	}

	stats, err := models.GetEventTimeline(campaignIDs, eventType, period)
	if err != nil {
		log.Errorf("Error getting event timeline: %v", err)
		JSONResponse(w, ErrorResponse{Message: "Error generating event timeline", Details: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// GetWorkHourDistributionReport handles requests for work hour distribution statistics.
// GET /api/stats/work_hours?campaign_ids=1,2,3&event_type=Clicked Link
func (as *Server) GetWorkHourDistributionReport(w http.ResponseWriter, r *http.Request) {
	campaignIDs, err := as.parseAndAuthorizeCampaignIDs(r, "campaign_ids")
	if err != nil {
		JSONResponse(w, ErrorResponse{Message: "Failed to parse or authorize campaign IDs", Details: err.Error()}, http.StatusBadRequest)
		return
	}
	eventType := r.URL.Query().Get("event_type")
	if eventType == "" {
		JSONResponse(w, ErrorResponse{Message: "event_type parameter is required"}, http.StatusBadRequest)
		return
	}

	stats, err := models.GetWorkHourDistribution(campaignIDs, eventType)
	if err != nil {
		log.Errorf("Error getting work hour distribution: %v", err)
		JSONResponse(w, ErrorResponse{Message: "Error generating work hour distribution", Details: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}
