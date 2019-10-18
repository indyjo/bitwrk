function showMandateDialog(type, articleId) {
	var dlg = $('#mandateModal');
	dlg.find("#perm-type-span").text(type);
	dlg.find("#perm-articleid-span").text(articleId);
	dlg.find("input[name='type']").val(type);
	dlg.find("input[name='articleid']").val(articleId);

	// Initialize previous values from stored cookies
	var tag = type + "-" + articleId;

	var priceInput = dlg.find("input[name='price']");
	var lastPrice = $.cookie("price:" + tag);
	if (lastPrice) {
		priceInput.val(lastPrice);
	}

	var tradesLeftInput = dlg.find("input[name='tradesleft']")
	var lastTradesLeft = $.cookie("tradesleft:" + tag);
	if (lastTradesLeft) {
		tradesLeftInput.val(lastTradesLeft);
	}

	var validMinutesInput = dlg.find("input[name='validminutes']")
	var lastValidMinutes = $.cookie("validminutes:" + tag);
	if (lastValidMinutes) {
		validMinutesInput.val(lastValidMinutes);
	}

	// Attach handler to form that stores cookie on submit
	var form = dlg.find("form");
	form.off('submit')
	form.on('submit', function() {
		$.cookie("price:" + tag, priceInput.val(), {
			'expires' : 21
		})
		$.cookie("tradesleft:" + tag, tradesLeftInput.val(), {
			'expires' : 21
		})
		$.cookie("validminutes:" + tag, validMinutesInput.val(), {
			'expires' : 21
		})
	});

	dlg.modal({
		'show' : true
	});
}
