$(document).ready(function() {
    var $campaignSelect = $('#campaign-select');
    var $campaignSelectorContainer = $('#campaign-selector-container');
    var $refreshButton = $('#refresh-stats-button');
    var $chartsContainer = $('#charts-container');
    let allCampaigns = []; // Store all campaign data {id, name}

    // Initialize select2
    $campaignSelect.select2({
        placeholder: "Select campaigns...",
        width: '100%',
        allowClear: true
    });

    // Fetch campaigns and populate the selector
    $.ajax({
        url: '/api/campaigns/',
        type: 'GET',
        dataType: 'json',
        success: function(campaignsData) {
            if (campaignsData && campaignsData.length > 0) {
                allCampaigns = campaignsData.map(function(c) { return { id: c.id, name: c.name }; });
                campaignsData.forEach(function(campaign) {
                    var option = new Option(campaign.name, campaign.id, false, false);
                    $campaignSelect.append(option);
                });
                $campaignSelect.trigger('change');
            } else {
                $campaignSelectorContainer.append('<p class="text-muted">No campaigns found.</p>');
            }
        },
        error: function(xhr, status, error) {
            console.error("Error fetching campaigns:", status, error);
            var errorMessage = '<div class="alert alert-danger">Error loading campaigns. Please try again later.</div>';
            $campaignSelectorContainer.children('label, select, .help-block, .btn').last().after(errorMessage);
        }
    });

    function displayError(containerId, message) {
        $('#' + containerId).html('<div class="alert alert-danger">' + message + '</div>');
    }

    function renderCombinedGeneralStatsChart(data, containerId) {
        if (!data) {
            displayError(containerId, "No data available for General Campaign Stats.");
            return;
        }
        Highcharts.chart(containerId, {
            chart: {
                type: 'bar'
            },
            title: {
                text: 'Campaign Statistics Summary' // Generic title
            },
            xAxis: {
                categories: ['Emails Sent', 'Opened', 'Clicked Link', 'Submitted Data', 'Reported'],
                title: {
                    text: null
                }
            },
            yAxis: {
                min: 0,
                title: {
                    text: 'Count',
                    align: 'high'
                },
                labels: {
                    overflow: 'justify'
                }
            },
            tooltip: {
                valueSuffix: ' events'
            },
            plotOptions: {
                bar: {
                    dataLabels: {
                        enabled: true
                    }
                }
            },
            credits: {
                enabled: false
            },
            series: [{
                name: 'Campaign Stats',
                data: [data.sent || 0, data.opened || 0, data.clicked || 0, data.submitted_data || 0, data.email_reported || 0]
            }]
        });
    }

    function renderCombinedEventTimelineChart(data, containerId) {
        if (!data || Object.keys(data).length === 0) {
            displayError(containerId, "No data available for Event Timeline.");
            return;
        }

        var seriesData = [];
        var relevantEvents = ["Opened Email", "Clicked Link", "Submitted Data"]; // Add more if needed

        relevantEvents.forEach(function(eventName) {
            if (data[eventName]) {
                seriesData.push({
                    name: eventName,
                    data: data[eventName].map(function(point) { // Ensure data format is [timestamp, count]
                        return [point[0], point[1]];
                    })
                });
            }
        });
        
        if (seriesData.length === 0) {
            displayError(containerId, "No relevant event types found in timeline data.");
            return;
        }

        Highcharts.chart(containerId, {
            chart: {
                type: 'line'
            },
            title: {
                text: 'Event Timeline' // Generic title
            },
            xAxis: {
                type: 'datetime',
                title: {
                    text: 'Date'
                }
            },
            yAxis: {
                title: {
                    text: 'Number of Events'
                },
                min: 0
            },
            tooltip: {
                shared: true,
                crosshairs: true
            },
            credits: {
                enabled: false
            },
            series: seriesData
        });
    }

    function renderCombinedUserAgentChart(data, containerId) {
        if (!data || Object.keys(data).length === 0) {
            displayError(containerId, "No data available for User-Agent Distribution.");
            return;
        }

        var parser = new UAParser();
        var browserCounts = {};
        for (var uaString in data) {
            if (data.hasOwnProperty(uaString)) {
                parser.setUA(uaString);
                var browserName = parser.getBrowser().name || "Unknown";
                browserCounts[browserName] = (browserCounts[browserName] || 0) + data[uaString];
            }
        }

        var seriesData = [];
        for (var name in browserCounts) {
            if (browserCounts.hasOwnProperty(name)) {
                seriesData.push({ name: name, y: browserCounts[name] });
            }
        }
        
        if (seriesData.length === 0) {
            displayError(containerId, "Could not parse any User-Agent data.");
            return;
        }

        Highcharts.chart(containerId, {
            chart: { type: 'pie' },
            title: { text: 'User-Agent Distribution (Browser)' }, // Generic title
            tooltip: { pointFormat: '{series.name}: <b>{point.percentage:.1f}%</b> ({point.y})' },
            plotOptions: {
                pie: {
                    allowPointSelect: true,
                    cursor: 'pointer',
                    dataLabels: {
                        enabled: true,
                        format: '<b>{point.name}</b>: {point.percentage:.1f} %'
                    }
                }
            },
            credits: { enabled: false },
            series: [{ name: 'Browsers', colorByPoint: true, data: seriesData }]
        });
    }

    function renderCombinedIPDistributionChart(data, containerId) {
        if (!data || Object.keys(data).length === 0) {
            displayError(containerId, "No data available for IP Address Distribution.");
            return;
        }
        var seriesData = [];
        // Optional: Implement Top N logic if needed, for now showing all
        var ips = Object.keys(data).map(function(key) {
            return { name: key, y: data[key] };
        });

        // Sort by count descending to easily pick Top N if desired
        ips.sort(function(a, b) { return b.y - a.y; });
        
        // For example, take Top 20 or all if less than 20
        var topN = 20;
        seriesData = ips.slice(0, topN);
        
        if (ips.length > topN) {
            var otherCount = ips.slice(topN).reduce(function(sum, item) { return sum + item.y; }, 0);
            if (otherCount > 0) {
                 seriesData.push({ name: 'Other IPs', y: otherCount });
            }
        }


        if (seriesData.length === 0) {
            displayError(containerId, "No IP address data to display.");
            return;
        }

        Highcharts.chart(containerId, {
            chart: { type: 'pie' },
            title: { text: 'IP Address Distribution (Top ' + topN + ')' }, // Generic title
            tooltip: { pointFormat: '{series.name}: <b>{point.percentage:.1f}%</b> ({point.y})' },
            plotOptions: {
                pie: {
                    allowPointSelect: true,
                    cursor: 'pointer',
                    dataLabels: {
                        enabled: true,
                        format: '<b>{point.name}</b>: {point.percentage:.1f} %'
                    }
                }
            },
            credits: { enabled: false },
            series: [{ name: 'IPs', colorByPoint: true, data: seriesData }]
        });
    }

    function renderCombinedHourlyDistributionChart(data, containerId) {
        if (!data || Object.keys(data).length === 0) {
            displayError(containerId, "No data available for Hourly Distribution.");
            return;
        }

        var workingHoursCount = 0;
        var offPeakHoursCount = 0;
        var workingHoursStart = 8; // 8 AM UTC
        var workingHoursEnd = 19;  // Up to 7 PM UTC (19:xx)

        for (var hourStr in data) {
            if (data.hasOwnProperty(hourStr)) {
                var hour = parseInt(hourStr, 10);
                var count = data[hourStr];
                if (hour >= workingHoursStart && hour <= workingHoursEnd) {
                    workingHoursCount += count;
                } else {
                    offPeakHoursCount += count;
                }
            }
        }
        
        if (workingHoursCount === 0 && offPeakHoursCount === 0) {
             displayError(containerId, "No hourly data to display after processing.");
            return;
        }

        var seriesData = [
            { name: 'Working Hours (8h-19h UTC)', y: workingHoursCount },
            { name: 'Off-Peak Hours', y: offPeakHoursCount }
        ];

        Highcharts.chart(containerId, {
            chart: { type: 'pie' },
            title: { text: 'Event Distribution by Hour' }, // Generic title
            tooltip: { pointFormat: '{series.name}: <b>{point.percentage:.1f}%</b> ({point.y})' },
            plotOptions: {
                pie: {
                    allowPointSelect: true,
                    cursor: 'pointer',
                    dataLabels: {
                        enabled: true,
                        format: '<b>{point.name}</b>: {point.percentage:.1f} %'
                    }
                }
            },
            credits: { enabled: false },
            series: [{ name: 'Events', colorByPoint: true, data: seriesData }]
        });
    }

    function loadCombinedCharts(campaignIDs) {
        // This function now only handles the "Combined Statistics" section
        if (!campaignIDs || campaignIDs.length === 0) {
            // If called with no IDs, ensure combined section is also cleared or has a message
            // This part might be redundant if handleRefresh clears $chartsContainer initially
            var combinedContainer = $('#combined-charts-section');
            if(combinedContainer.length === 0) {
                 $chartsContainer.prepend('<div id="combined-charts-section"></div>');
                 combinedContainer = $('#combined-charts-section');
            }
            combinedContainer.html('<p class="text-info">Select one or more campaigns to view statistics.</p>');
            return;
        }

        var combinedSectionHtml = `
            <div id="combined-charts-section">
                <h2>Combined Statistics for Selected Campaigns (${campaignIDs.length})</h2>
                <div class="row">
                    <div class="col-lg-12 mb-4">
                        <div id="combined-general-stats-chart" style="height: 400px;"></div>
                    </div>
                </div>
                <div class="row">
                    <div class="col-lg-12 mb-4">
                        <div id="combined-event-timeline-chart" style="height: 400px;"></div>
                    </div>
                </div>
                <div class="row">
                    <div class="col-lg-6 mb-4">
                        <div id="combined-user-agent-chart" style="height: 400px;"></div>
                    </div>
                    <div class="col-lg-6 mb-4">
                        <div id="combined-hourly-distribution-chart" style="height: 400px;"></div>
                    </div>
                </div>
                <div class="row">
                    <div class="col-lg-12 mb-4">
                        <div id="combined-ip-distribution-chart" style="height: 400px;"></div>
                    </div>
                </div>
            </div>`;
        
        // Ensure #combined-charts-section exists or create it
        var $combinedChartsSection = $('#combined-charts-section');
        if ($combinedChartsSection.length === 0) {
            $chartsContainer.append('<div id="combined-charts-section"></div>');
            $combinedChartsSection = $('#combined-charts-section');
        }
        $combinedChartsSection.html(combinedSectionHtml);


        var campaignIDsStr = campaignIDs.join(',');

        // Fetch and render Aggregated Summary for combined view
        $('#combined-general-stats-chart').html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
        $.ajax({
            url: '/api/campaigns/stats/aggregated_summary?campaign_ids=' + campaignIDsStr,
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                renderCombinedGeneralStatsChart(data, 'combined-general-stats-chart');
            },
            error: function(xhr, status, error) {
                console.error("Error fetching aggregated summary:", status, error);
                displayError('combined-general-stats-chart', 'Error loading aggregated summary: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error));
            }
        });

        // Fetch and render Event Timeline
        $('#combined-event-timeline-chart').html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
        $.ajax({
            url: '/api/campaigns/stats/timeline?campaign_ids=' + campaignIDsStr, // Ensure this is the correct endpoint
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                renderCombinedEventTimelineChart(data, 'combined-event-timeline-chart');
            },
            error: function(xhr, status, error) {
                console.error("Error fetching event timeline:", status, error);
                displayError('combined-event-timeline-chart', 'Error loading event timeline: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error));
            }
        });

        // Fetch and render User Agent Distribution
        $('#combined-user-agent-chart').html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
        $.ajax({
            url: '/api/campaigns/stats/user_agents?campaign_ids=' + campaignIDsStr,
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                renderCombinedUserAgentChart(data, 'combined-user-agent-chart');
            },
            error: function(xhr, status, error) {
                console.error("Error fetching user agent data:", status, error);
                displayError('combined-user-agent-chart', 'Error loading User-Agent distribution: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error));
            }
        });

        // Fetch and render IP Distribution
        $('#combined-ip-distribution-chart').html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
        $.ajax({
            url: '/api/campaigns/stats/ip_distribution?campaign_ids=' + campaignIDsStr,
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                renderCombinedIPDistributionChart(data, 'combined-ip-distribution-chart');
            },
            error: function(xhr, status, error) {
                console.error("Error fetching IP distribution data:", status, error);
                displayError('combined-ip-distribution-chart', 'Error loading IP distribution: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error));
            }
        });
        
        // Fetch and render Hourly Distribution
        $('#combined-hourly-distribution-chart').html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
        $.ajax({
            url: '/api/campaigns/stats/hourly_distribution?campaign_ids=' + campaignIDsStr,
            type: 'GET',
            dataType: 'json',
            success: function(data) {
                renderCombinedHourlyDistributionChart(data, 'combined-hourly-distribution-chart');
            },
            error: function(xhr, status, error) {
                console.error("Error fetching hourly distribution data:", status, error);
                displayError('combined-hourly-distribution-chart', 'Error loading hourly distribution: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error));
            }
        });
    }

    // Event listener for campaign selection change and refresh button
    function handleRefresh() {
        var selectedIDs = $campaignSelect.val();
        if (selectedIDs === null) {
            selectedIDs = [];
        }
        console.log("Loading charts for campaign IDs:", selectedIDs);
        
        $chartsContainer.empty(); // Clear everything before loading new views

        if (selectedIDs && selectedIDs.length > 0) {
            loadCombinedCharts(selectedIDs); // Load combined view first

            if (selectedIDs.length > 1) { // Only load individual if more than one selected
                loadIndividualCampaignCharts(selectedIDs);
            }
        } else {
            // No campaigns selected, display a message
            $chartsContainer.html('<p class="text-info">Please select one or more campaigns to view statistics.</p>');
        }
    }

    function loadIndividualCampaignCharts(campaignIDs) {
        $chartsContainer.append('<hr class="my-4">'); // Separator

        campaignIDs.forEach(function(campaignID) {
            var campaign = allCampaigns.find(function(c) { return c.id == campaignID; });
            var campaignName = campaign ? campaign.name : 'Campaign ' + campaignID;
            var safeCampaignID = String(campaignID).replace(/[^a-zA-Z0-9-_]/g, ''); // Sanitize for use in ID

            var individualCampaignSectionHtml = `
                <div id="individual-campaign-${safeCampaignID}-section" class="individual-campaign-charts mb-5">
                    <h3>Statistics for: ${campaignName}</h3>
                    <div class="row">
                        <div class="col-lg-12 mb-4">
                            <div id="campaign-${safeCampaignID}-general-stats-chart" style="height: 400px;"></div>
                        </div>
                    </div>
                    <div class="row">
                        <div class="col-lg-12 mb-4">
                            <div id="campaign-${safeCampaignID}-event-timeline-chart" style="height: 400px;"></div>
                        </div>
                    </div>
                    <div class="row">
                        <div class="col-lg-6 mb-4">
                            <div id="campaign-${safeCampaignID}-user-agent-chart" style="height: 400px;"></div>
                        </div>
                        <div class="col-lg-6 mb-4">
                            <div id="campaign-${safeCampaignID}-hourly-distribution-chart" style="height: 400px;"></div>
                        </div>
                    </div>
                    <div class="row">
                        <div class="col-lg-12 mb-4">
                            <div id="campaign-${safeCampaignID}-ip-distribution-chart" style="height: 400px;"></div>
                        </div>
                    </div>
                </div>
            `;
            $chartsContainer.append(individualCampaignSectionHtml);

            // API calls for this individual campaign
            const singleCampaignIDStr = String(campaignID);

            // General Stats
            $(`#campaign-${safeCampaignID}-general-stats-chart`).html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
            $.ajax({
                url: '/api/campaigns/stats/aggregated_summary?campaign_ids=' + singleCampaignIDStr,
                type: 'GET', dataType: 'json',
                success: function(data) { renderCombinedGeneralStatsChart(data, `campaign-${safeCampaignID}-general-stats-chart`); },
                error: function(xhr, status, error) { displayError(`campaign-${safeCampaignID}-general-stats-chart`, 'Error loading summary: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error)); }
            });

            // Event Timeline
            $(`#campaign-${safeCampaignID}-event-timeline-chart`).html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
            $.ajax({
                url: '/api/campaigns/stats/timeline?campaign_ids=' + singleCampaignIDStr,
                type: 'GET', dataType: 'json',
                success: function(data) { renderCombinedEventTimelineChart(data, `campaign-${safeCampaignID}-event-timeline-chart`); },
                error: function(xhr, status, error) { displayError(`campaign-${safeCampaignID}-event-timeline-chart`, 'Error loading timeline: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error)); }
            });

            // User Agents
            $(`#campaign-${safeCampaignID}-user-agent-chart`).html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
            $.ajax({
                url: '/api/campaigns/stats/user_agents?campaign_ids=' + singleCampaignIDStr,
                type: 'GET', dataType: 'json',
                success: function(data) { renderCombinedUserAgentChart(data, `campaign-${safeCampaignID}-user-agent-chart`); },
                error: function(xhr, status, error) { displayError(`campaign-${safeCampaignID}-user-agent-chart`, 'Error loading user agents: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error)); }
            });

            // IP Distribution
            $(`#campaign-${safeCampaignID}-ip-distribution-chart`).html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
            $.ajax({
                url: '/api/campaigns/stats/ip_distribution?campaign_ids=' + singleCampaignIDStr,
                type: 'GET', dataType: 'json',
                success: function(data) { renderCombinedIPDistributionChart(data, `campaign-${safeCampaignID}-ip-distribution-chart`); },
                error: function(xhr, status, error) { displayError(`campaign-${safeCampaignID}-ip-distribution-chart`, 'Error loading IP distribution: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error)); }
            });

            // Hourly Distribution
            $(`#campaign-${safeCampaignID}-hourly-distribution-chart`).html('<div class="text-center"><i class="fa fa-spinner fa-spin fa-2x"></i> Loading...</div>');
            $.ajax({
                url: '/api/campaigns/stats/hourly_distribution?campaign_ids=' + singleCampaignIDStr,
                type: 'GET', dataType: 'json',
                success: function(data) { renderCombinedHourlyDistributionChart(data, `campaign-${safeCampaignID}-hourly-distribution-chart`); },
                error: function(xhr, status, error) { displayError(`campaign-${safeCampaignID}-hourly-distribution-chart`, 'Error loading hourly distribution: ' + (xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : error)); }
            });
        });
    }

    $campaignSelect.on('change', handleRefresh);
    $refreshButton.on('click', handleRefresh);

    // Initial load (optional, if you want charts to load when page first loads with pre-selected campaigns or all)
    // For now, let's wait for user interaction.
    // handleRefresh(); 
});
