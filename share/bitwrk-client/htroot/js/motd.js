function showAlertBox(alertbox, alertcontent, alertclass, htmlcontent) {
	alertcontent.html(htmlcontent);

	alertbox.removeClass("alert-info");
	alertbox.removeClass("alert-warning");
	alertbox.removeClass("alert-danger");
	alertbox.removeClass("alert-success");
	alertbox.addClass(alertclass);
	alertbox.fadeIn();
}

function newMessageUpdater(alertbox, content) {
	var lastval = $.cookie("lastmotd");
	var currentval = "";
	alertbox.bind('closed.bs.alert', function() {
		lastval = currentval;
		$.cookie("lastmotd", currentval, {
			'expires' : 1,
			'path' : '/',
		});
	});
	return function() {
		var xhr = new XMLHttpRequest();
		xhr.onreadystatechange = function() {
			if (xhr.readyState === 4 && xhr.status == 200) {
				currentval = xhr.responseText;
				if (currentval != lastval) {
					var motd = JSON.parse(currentval)

					var alertclass;
					if (motd.Error) {
						alertclass = "alert-danger";
					} else if (motd.Warning) {
						alertclass = "alert-warning";
					} else if (motd.Success) {
						alertclass = "alert-success";
					} else {
						alertclass = "alert-info";
					}

					showAlertBox(alertbox, content, alertclass, motd.Text)
				}
			}
		};
		xhr.open("GET", "/motd");
		xhr.setRequestHeader("Accept", "application/json");
		xhr.send();
	};
}