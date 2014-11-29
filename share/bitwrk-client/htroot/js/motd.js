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
					content.html(motd.Text);

					alertbox.removeClass("alert-info");
					alertbox.removeClass("alert-warning");
					alertbox.removeClass("alert-danger");
					alertbox.removeClass("alert-success");
					if (motd.Error) {
						alertbox.addClass("alert-danger");
					} else if (motd.Warning) {
						alertbox.addClass("alert-warning");
					} else if (motd.Success) {
						alertbox.addClass("alert-success");
					} else {
						alertbox.addClass("alert-info");
					}
					alertbox.fadeIn();
				}
			}
		};
		xhr.open("GET", "/motd");
		xhr.setRequestHeader("Accept", "application/json");
		xhr.send();
	};
}