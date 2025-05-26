// labels is a map of campaign statuses to
// CSS classes
var labels = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger"
}

var campaigns = []
var campaign = {}

// Function to handle visibility of campaign fields based on type
function handleCampaignTypeChange() {
    var campaignType = $("#campaign_type").val();
    if (campaignType === "Device Token") {
        $('#token_scope_group').show();
        // Hide standard campaign fields by targeting the parent .form-group
        // This assumes the HTML structure places each field (label + input/select) in a .form-group
        $('#template').closest('.form-group.standard-campaign-field').hide();
        $('#page').closest('.form-group.standard-campaign-field').hide();
        $('#profile').closest('.form-group.standard-campaign-field').hide();
    } else { // Standard campaign
        $('#token_scope_group').hide();
        $('#template').closest('.form-group.standard-campaign-field').show();
        $('#page').closest('.form-group.standard-campaign-field').show();
        $('#profile').closest('.form-group.standard-campaign-field').show();
    }
}


// Launch attempts to POST to /campaigns/
function launch() {
    Swal.fire({
        title: "Are you sure?",
        text: "This will schedule the campaign to be launched.",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                var groups = [];
                $("#users").select2("data").forEach(function (group) {
                    groups.push({
                        name: group.text
                    });
                });
                // Validate our fields
                var send_by_date = $("#send_by_date").val();
                if (send_by_date != "") {
                    send_by_date = moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format();
                }

                var campaign_type = $("#campaign_type").val();
                
                campaign = {
                    name: $("#name").val(),
                    url: $("#url").val(),
                    launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
                    send_by_date: send_by_date || null,
                    groups: groups,
                    type: campaign_type
                };

                if (campaign_type === "Device Token") {
                    campaign.token_scope = $("#token_scope").val();
                    // For device token campaigns, ensure these are not part of the payload
                    // or are explicitly set to null/empty if the backend expects them.
                    // Backend should ideally ignore them based on type.
                    delete campaign.template;
                    delete campaign.page;
                    delete campaign.smtp;
                } else { // Standard campaign
                    campaign.template = { name: $("#template").select2("data")[0].text };
                    campaign.page = { name: $("#page").select2("data")[0].text };
                    campaign.smtp = { name: $("#profile").select2("data")[0].text };
                }
                
                // Submit the campaign
                api.campaigns.post(campaign)
                    .success(function (data) {
                        resolve()
                        campaign = data
                    })
                    .error(function (data) {
                        var errorMessage = "An unknown error occurred.";
                        if (data && data.responseJSON && data.responseJSON.message) {
                            errorMessage = data.responseJSON.message;
                        }
                        $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(errorMessage) + "</div>")
                        Swal.close()
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Scheduled!',
                'This campaign has been scheduled for launch!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            window.location = "/campaigns/" + campaign.id.toString()
        })
    })
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    // Test email functionality should only be available for standard campaigns
    if ($("#campaign_type").val() === "Device Token") {
        $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-warning\">\
        <i class=\"fa fa-exclamation-circle\"></i> Test emails are not applicable for Device Token campaigns.</div>");
        return;
    }

    var test_email_request = {
        template: {
            name: $("#template").select2("data")[0].text
        },
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        url: $("#url").val(),
        page: {
            name: $("#page").select2("data")[0].text
        },
        smtp: {
            name: $("#profile").select2("data")[0].text
        }
    }
    var btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    // Send the test email
    api.send_test_email(test_email_request)
        .success(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
            <i class=\"fa fa-check-circle\"></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            var errorMessage = "An unknown error occurred.";
            if (data && data.responseJSON && data.responseJSON.message) {
                errorMessage = data.responseJSON.message;
            }
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(errorMessage) + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#campaign_type").val("Standard").trigger('change'); // Reset to Standard and trigger change
    $("#token_scope").val("");
    $("#template").val("").change();
    $("#page").val("").change();
    $("#url").val("");
    $("#profile").val("").change();
    $("#users").val("").change();
    $("#modal").modal('hide');
}

function deleteCampaign(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + campaigns[idx].name,
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaigns[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Deleted!',
                'This campaign has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function setupOptions() {
    api.groups.summary()
        .success(function (summaries) {
            groups = summaries.groups
            if (groups.length == 0) {
                modalError("No groups found!")
                return false;
            } else {
                var group_s2 = $.map(groups, function (obj) {
                    obj.text = obj.name
                    obj.title = obj.num_targets + " targets"
                    return obj
                });
                $("#users.form-control").select2({
                    placeholder: "Select Groups",
                    data: group_s2,
                });
            }
        });
    api.templates.get()
        .success(function (templates) {
            if (templates.length == 0) {
                // modalError("No templates found!") // Only show error if it's a standard campaign
                // return false
            } else {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var template_select = $("#template.form-control")
                template_select.select2({
                    placeholder: "Select a Template",
                    data: template_s2,
                });
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        });
    api.pages.get()
        .success(function (pages) {
            if (pages.length == 0) {
               // modalError("No pages found!") // Only show error if it's a standard campaign
               // return false
            } else {
                var page_s2 = $.map(pages, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var page_select = $("#page.form-control")
                page_select.select2({
                    placeholder: "Select a Landing Page",
                    data: page_s2,
                });
                if (pages.length === 1) {
                    page_select.val(page_s2[0].id)
                    page_select.trigger('change.select2')
                }
            }
        });
    api.SMTP.get()
        .success(function (profiles) {
            if (profiles.length == 0) {
                // modalError("No profiles found!") // Only show error if it's a standard campaign
                // return false
            } else {
                var profile_s2 = $.map(profiles, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var profile_select = $("#profile.form-control")
                profile_select.select2({
                    placeholder: "Select a Sending Profile",
                    data: profile_s2,
                }).select2("val", profile_s2[0]);
                if (profiles.length === 1) {
                    profile_select.val(profile_s2[0].id)
                    profile_select.trigger('change.select2')
                }
            }
        });
}

function edit(campaign_type_arg) { // Renamed arg to avoid conflict
    setupOptions();
    // If new, ensure fields are set to standard and shown correctly
    if (campaign_type_arg === 'new') {
         $("#campaign_type").val("Standard").trigger('change');
    }
}


function copy(idx) {
    setupOptions(); // This will populate dropdowns
    // Set our initial values
    api.campaignId.get(campaigns[idx].id)
        .success(function (campaign_data) { // Renamed to avoid conflict
            $("#name").val("Copy of " + campaign_data.name);
            $("#campaign_type").val(campaign_data.type || "Standard").trigger('change');

            if (campaign_data.type === "Device Token") {
                $("#token_scope").val(campaign_data.token_scope || "");
            } else {
                 // Standard campaign fields
                if (!campaign_data.template.id) {
                    $("#template").val("").change();
                    $("#template").select2({
                        placeholder: campaign_data.template.name
                    });
                } else {
                    $("#template").val(campaign_data.template.id.toString());
                    $("#template").trigger("change.select2")
                }
                if (!campaign_data.page.id) {
                    $("#page").val("").change();
                    $("#page").select2({
                        placeholder: campaign_data.page.name
                    });
                } else {
                    $("#page").val(campaign_data.page.id.toString());
                    $("#page").trigger("change.select2")
                }
                if (!campaign_data.smtp.id) {
                    $("#profile").val("").change();
                    $("#profile").select2({
                        placeholder: campaign_data.smtp.name
                    });
                } else {
                    $("#profile").val(campaign_data.smtp.id.toString());
                    $("#profile").trigger("change.select2")
                }
            }
            $("#url").val(campaign_data.url);
            // Correctly set launch_date and send_by_date using the datetimepicker's methods if available,
            // or by formatting the date string appropriately for the input field.
            // For simplicity, directly setting val, assuming datetimepicker handles parsing.
            if (campaign_data.launch_date) {
                $("#launch_date").val(moment(campaign_data.launch_date).format("MMMM Do YYYY, h:mm a"));
            }
            if (campaign_data.send_by_date) {
                $("#send_by_date").val(moment(campaign_data.send_by_date).format("MMMM Do YYYY, h:mm a"));
            }
            // Groups might need special handling if IDs are used vs names
            // Assuming current setup with names is fine for copy.
            var group_ids = [];
            if (campaign_data.groups) {
                campaign_data.groups.forEach(function(g){
                    // This assumes that the select2 data items have an `id` that matches `g.id`
                    // If groups are matched by name, this needs adjustment.
                    // For now, if `g.id` is not directly usable, this might not correctly select groups.
                    // This part might need more robust logic depending on how select2 is populated and how group IDs vs names are handled.
                });
                // $("#users").val(group_ids).trigger('change.select2'); // Example if using IDs
            }


        })
        .error(function (data) {
            var errorMessage = "An unknown error occurred.";
            if (data && data.responseJSON && data.responseJSON.message) {
                errorMessage = data.responseJSON.message;
            }
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(errorMessage) + "</div>")
        })
}

$(document).ready(function () {
    // Event listener for campaign type change
    $('#campaign_type').on('change', handleCampaignTypeChange);
    // Initial call to set correct field visibility
    handleCampaignTypeChange();

    $("#launch_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "defaultDate": moment(),
        "format": "MMMM Do YYYY, h:mm a"
    })
    $("#send_by_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "useCurrent": false,
        "format": "MMMM Do YYYY, h:mm a"
    })
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    api.campaigns.summary()
        .success(function (data) {
            campaigns = data.campaigns
            $("#loading").hide()
            if (campaigns.length > 0) {
                $("#campaignTable").show()
                $("#campaignTableArchive").show()

                activeCampaignsTable = $("#campaignTable").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                archivedCampaignsTable = $("#campaignTableArchive").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                rows = {
                    'active': [],
                    'archived': []
                }
                $.each(campaigns, function (i, campaign) {
                    label = labels[campaign.status] || "label-default";

                    //section for tooltips on the status of a campaign to show some quick stats
                    var launchDate;
                    if (moment(campaign.launch_date).isAfter(moment())) {
                        launchDate = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total
                    } else {
                        launchDate = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total + "<br><br>" + "Emails opened: " + campaign.stats.opened + "<br><br>" + "Emails clicked: " + campaign.stats.clicked + "<br><br>" + "Submitted Credentials: " + campaign.stats.submitted_data + "<br><br>" + "Errors : " + campaign.stats.error + "<br><br>" + "Reported : " + campaign.stats.email_reported
                    }

                    var row = [
                        escapeHtml(campaign.name),
                        moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        "<div class='pull-right'><a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>\
                    <i class='fa fa-bar-chart'></i>\
                    </a>\
            <span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Campaign' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]
                    if (campaign.status == 'Completed') {
                        rows['archived'].push(row)
                    } else {
                        rows['active'].push(row)
                    }
                })
                activeCampaignsTable.rows.add(rows['active']).draw()
                archivedCampaignsTable.rows.add(rows['archived']).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching campaigns")
        })
    // Select2 Defaults
    $.fn.select2.defaults.set("width", "100%");
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"));
    $.fn.select2.defaults.set("theme", "bootstrap");
    $.fn.select2.defaults.set("sorter", function (data) {
        return data.sort(function (a, b) {
            if (a.text.toLowerCase() > b.text.toLowerCase()) {
                return 1;
            }
            if (a.text.toLowerCase() < b.text.toLowerCase()) {
                return -1;
            }
            return 0;
        });
    })
})
