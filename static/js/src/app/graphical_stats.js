$(document).ready(function() {
    // Initialize Select2 for the campaign selector
    $('#campaign_selector_stats').select2({
        placeholder: "Select one or more campaigns",
        width: '100%',
        allowClear: true
    });

    // Populate Campaign Selector
    api.campaigns.summary()
        .success(function(data) {
            if (data.campaigns && data.campaigns.length > 0) {
                var campaigns = data.campaigns.sort(function(a, b) {
                    return new Date(b.created_date) - new Date(a.created_date);
                });
                campaigns.forEach(function(campaign) {
                    var option = new Option(campaign.name, campaign.id, false, false);
                    $('#campaign_selector_stats').append(option);
                });
                $('#campaign_selector_stats').trigger('change');
            } else {
                $('#campaign_selector_stats').empty().append('<option value="">No campaigns found</option>');
            }
        })
        .error(function() {
            $('#flashes').empty().append('<div class="alert alert-danger">Error fetching campaigns. Please try again later.</div>');
            $('#campaign_selector_stats').empty().append('<option value="">Error fetching campaigns</option>');
        });

    // Event Listener for "Generate Statistics" Button
    $('#generate_stats_button').on('click', function() {
        var selectedCampaignIDs = $('#campaign_selector_stats').val();

        // Clear previous stats
        $('#campaign_stats_container').empty();
        $('#combined_stats_container').empty().hide();
        $('#flashes').empty();

        if (!selectedCampaignIDs || selectedCampaignIDs.length === 0) {
            $('#flashes').append('<div class="alert alert-warning">Please select at least one campaign.</div>');
            return;
        }

        // Generate stats for each selected campaign individually
        selectedCampaignIDs.forEach(function(campaignId) {
            var campaign = campaigns.find(c => c.id.toString() === campaignId); // Assumes `campaigns` is available from summary call
            var campaignName = campaign ? campaign.name : "Campaign " + campaignId;
            
            var campaignContainerId = `stats_for_campaign_${campaignId}`;
            $('#campaign_stats_container').append(`<div id="${campaignContainerId}" class="campaign-charts panel panel-default"><div class="panel-heading"><h3>${escapeHtml(campaignName)}</h3></div><div class="panel-body"></div></div>`);
            
            fetchAndRenderAllChartsForCampaign(campaignId, `#${campaignContainerId} .panel-body`);
        });

        // Generate combined stats if more than one campaign is selected
        if (selectedCampaignIDs.length > 1) {
            $('#combined_stats_container').show();
             $('#combined_stats_container').append(`<div class="panel panel-default"><div class="panel-heading"><h2>Combined Statistics for Selected Campaigns</h2></div><div class="panel-body"></div></div>`);
            fetchAndRenderAllChartsForCampaign(selectedCampaignIDs.join(','), '#combined_stats_container .panel-body', true);
        }
    });
});

// Helper to escape HTML
function escapeHtml(unsafe) {
    return unsafe
         .replace(/&/g, "&amp;")
         .replace(/</g, "&lt;")
         .replace(/>/g, "&gt;")
         .replace(/"/g, "&quot;")
         .replace(/'/g, "&#039;");
 }

// Main function to fetch and render all charts for a given set of campaign IDs
function fetchAndRenderAllChartsForCampaign(campaignIDs, targetContainerSelector, isCombined = false) {
    var campaignIdParam = Array.isArray(campaignIDs) ? campaignIDs.join(',') : campaignIDs;
    var containerPrefix = isCombined ? "combined_" : `campaign_${campaignIDs.toString().replace(/,/g, '_')}_`;

    // 1. General Stats (Bar Chart)
    fetchGeneralStats(campaignIdParam, targetContainerSelector, containerPrefix + "general_stats");

    // Event types for other stats
    var eventTypes = ["Opened", "Clicked", "Submitted Data"]; 

    eventTypes.forEach(function(eventType) {
        var safeEventType = eventType.replace(/\s+/g, '_'); // e.g., Submitted_Data
        // 2. User-Agent Stats (Pie Chart - OS)
        fetchUserAgentStats(campaignIdParam, eventType, targetContainerSelector, containerPrefix + "ua_os_" + safeEventType);
        // 3. IP Stats (Pie Chart) - May need grouping for many IPs
        fetchIPStats(campaignIdParam, eventType, targetContainerSelector, containerPrefix + "ip_" + safeEventType);
        // 4. Event Timeline (Line Chart) - Defaulting to daily
        fetchEventTimeline(campaignIdParam, eventType, "daily", targetContainerSelector, containerPrefix + "timeline_" + safeEventType);
        // 5. Work Hour Distribution (Pie Chart)
        fetchWorkHourDistribution(campaignIdParam, eventType, targetContainerSelector, containerPrefix + "workhour_" + safeEventType);
    });
}

// --- Data Fetching Functions ---
function fetchGeneralStats(campaignIDs, targetContainer, chartDivId) {
    var endpoint = `/api/stats/campaigns/combined?campaign_ids=${campaignIDs}`;
    $.getJSON(endpoint)
        .done(function(data) {
            renderGeneralStatsChart(data, targetContainer, chartDivId);
        })
        .fail(function(jqXHR, textStatus, errorThrown) {
            console.error("Error fetching general stats:", textStatus, errorThrown);
            $(targetContainer).append(`<div id="${chartDivId}" class="chart-placeholder alert alert-danger">Error loading general stats.</div>`);
        });
}

function fetchUserAgentStats(campaignIDs, eventType, targetContainer, chartDivId) {
    var endpoint = `/api/stats/user_agent?campaign_ids=${campaignIDs}&event_type=${encodeURIComponent(eventType)}`;
    $.getJSON(endpoint)
        .done(function(data) {
            renderUserAgentChart(data, eventType, targetContainer, chartDivId);
        })
        .fail(function(jqXHR, textStatus, errorThrown) {
            console.error(`Error fetching UA stats for ${eventType}:`, textStatus, errorThrown);
            $(targetContainer).append(`<div id="${chartDivId}" class="chart-placeholder alert alert-danger">Error loading User-Agent stats for ${escapeHtml(eventType)}.</div>`);
        });
}

function fetchIPStats(campaignIDs, eventType, targetContainer, chartDivId) {
    var endpoint = `/api/stats/ip?campaign_ids=${campaignIDs}&event_type=${encodeURIComponent(eventType)}`;
     $.getJSON(endpoint)
        .done(function(data) {
            renderIPStatsChart(data, eventType, targetContainer, chartDivId);
        })
        .fail(function(jqXHR, textStatus, errorThrown) {
            console.error(`Error fetching IP stats for ${eventType}:`, textStatus, errorThrown);
            $(targetContainer).append(`<div id="${chartDivId}" class="chart-placeholder alert alert-danger">Error loading IP stats for ${escapeHtml(eventType)}.</div>`);
        });
}

function fetchEventTimeline(campaignIDs, eventType, period, targetContainer, chartDivId) {
    var endpoint = `/api/stats/timeline?campaign_ids=${campaignIDs}&event_type=${encodeURIComponent(eventType)}&period=${period}`;
    $.getJSON(endpoint)
        .done(function(data) {
            renderEventTimelineChart(data, eventType, period, targetContainer, chartDivId);
        })
        .fail(function(jqXHR, textStatus, errorThrown) {
            console.error(`Error fetching timeline for ${eventType}:`, textStatus, errorThrown);
            $(targetContainer).append(`<div id="${chartDivId}" class="chart-placeholder alert alert-danger">Error loading timeline for ${escapeHtml(eventType)}.</div>`);
        });
}

function fetchWorkHourDistribution(campaignIDs, eventType, targetContainer, chartDivId) {
    var endpoint = `/api/stats/work_hours?campaign_ids=${campaignIDs}&event_type=${encodeURIComponent(eventType)}`;
    $.getJSON(endpoint)
        .done(function(data) {
            renderWorkHourDistributionChart(data, eventType, targetContainer, chartDivId);
        })
        .fail(function(jqXHR, textStatus, errorThrown) {
            console.error(`Error fetching work hour distribution for ${eventType}:`, textStatus, errorThrown);
            $(targetContainer).append(`<div id="${chartDivId}" class="chart-placeholder alert alert-danger">Error loading work hour distribution for ${escapeHtml(eventType)}.</div>`);
        });
}

// --- Chart Rendering Functions ---
function renderGeneralStatsChart(apiData, targetContainer, chartDivId) {
    $(targetContainer).append(`<h4>Overall Campaign Performance</h4><div id="${chartDivId}" class="ct-chart ct-golden-section chart-placeholder"></div>`);
    if (!apiData || apiData.total === 0 && apiData.sent === 0 ) { // Check if data is essentially empty
        $(`#${chartDivId}`).html('<div class="alert alert-info">No general stats data available.</div>');
        return;
    }
    var data = {
        labels: ['Sent', 'Opened', 'Clicked', 'Submitted Data', 'Email Reported'],
        series: [[
            apiData.sent || 0, 
            apiData.opened || 0, 
            apiData.clicked || 0, 
            apiData.submitted_data || 0,
            apiData.email_reported || 0
        ]]
    };
    var options = {
        seriesBarDistance: 10,
        axisY: {
            onlyInteger: true
        }
    };
    new Chartist.Bar(`#${chartDivId}`, data, options);
}

function renderUserAgentChart(apiData, eventType, targetContainer, chartDivId) {
    $(targetContainer).append(`<h4>User Agents (${escapeHtml(eventType)}) - OS</h4><div id="${chartDivId}" class="ct-chart ct-golden-section chart-placeholder"></div>`);
    if (!apiData || apiData.length === 0) {
        $(`#${chartDivId}`).html(`<div class="alert alert-info">No User-Agent data for ${escapeHtml(eventType)}.</div>`);
        return;
    }
    var labels = apiData.map(item => item.os);
    var series = apiData.map(item => item.count);
    var data = { labels: labels, series: series };
    var options = { labelInterpolationFnc: function(value, idx) { return value + ' (' + series[idx] + ')'; }};
    new Chartist.Pie(`#${chartDivId}`, data, options);
}

function renderIPStatsChart(apiData, eventType, targetContainer, chartDivId) {
    $(targetContainer).append(`<h4>Top IPs (${escapeHtml(eventType)})</h4><div id="${chartDivId}" class="ct-chart ct-golden-section chart-placeholder"></div>`);
    if (!apiData || apiData.length === 0) {
        $(`#${chartDivId}`).html(`<div class="alert alert-info">No IP data for ${escapeHtml(eventType)}.</div>`);
        return;
    }
    // Logic to group smaller percentages into "Other" might be needed for very fragmented data
    var maxIPsToShow = 10;
    var labels = apiData.slice(0, maxIPsToShow).map(item => item.ip_address);
    var series = apiData.slice(0, maxIPsToShow).map(item => item.count);
    if (apiData.length > maxIPsToShow) {
        labels.push("Other");
        var otherCount = 0;
        for(var i = maxIPsToShow; i < apiData.length; i++) {
            otherCount += apiData[i].count;
        }
        series.push(otherCount);
    }
    var data = { labels: labels, series: series };
     var options = { labelInterpolationFnc: function(value, idx) { return value + ' (' + series[idx] + ')'; }};
    new Chartist.Pie(`#${chartDivId}`, data, options);
}

function renderEventTimelineChart(apiData, eventType, period, targetContainer, chartDivId) {
    $(targetContainer).append(`<h4>${escapeHtml(eventType)} Timeline (${escapeHtml(period)})</h4><div id="${chartDivId}" class="ct-chart ct-golden-section chart-placeholder"></div>`);
    if (!apiData || apiData.length === 0) {
        $(`#${chartDivId}`).html(`<div class="alert alert-info">No timeline data for ${escapeHtml(eventType)}.</div>`);
        return;
    }
    var labels = apiData.map(item => item.time);
    var series = [apiData.map(item => item.count)];
    var data = { labels: labels, series: series};
    var options = {
      axisX: {
        labelInterpolationFnc: function(value, index) {
          // Show fewer labels if there are many, e.g., every Nth label
          return index % Math.ceil(labels.length / 15) === 0 ? value : null;
        }
      },
      axisY: { onlyInteger: true }
    };
    new Chartist.Line(`#${chartDivId}`, data, options);
}

function renderWorkHourDistributionChart(apiData, eventType, targetContainer, chartDivId) {
    $(targetContainer).append(`<h4>Work Hour Distribution (${escapeHtml(eventType)})</h4><div id="${chartDivId}" class="ct-chart ct-golden-section chart-placeholder"></div>`);
    if (!apiData || (apiData["Work Hours"] === 0 && apiData["Off Hours"] === 0)) {
         $(`#${chartDivId}`).html(`<div class="alert alert-info">No work hour data for ${escapeHtml(eventType)}.</div>`);
        return;
    }
    var labels = ["Work Hours", "Off Hours"];
    var series = [apiData["Work Hours"] || 0, apiData["Off Hours"] || 0];
    var data = { labels: labels, series: series };
    var options = { labelInterpolationFnc: function(value, idx) { return value + ' (' + series[idx] + ')'; }};
    new Chartist.Pie(`#${chartDivId}`, data, options);
}
