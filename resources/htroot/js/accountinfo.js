function setAccountInfo(node, accountjson) {
	var account = JSON.parse(accountjson);

	$(node).find(".available span").text(account.Account.Available)
	$(node).find(".blocked span").text(account.Account.Blocked)
}

function updateAccountInfo() {
	var xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState === 4 && xhr.status == 200) {
			setAccountInfo(document.getElementById("account-info"),
					xhr.responseText);
		}
	};
	xhr.open("GET", "/myaccount");
	xhr.setRequestHeader("Accept", "application/json");
	xhr.send();
}

function updateDepositInfo() {
	var xhr = new XMLHttpRequest();
	xhr.onreadystatechange = function() {
		if (xhr.readyState !== 4) {
			return;
		}

		$(".depositaddress-yes").addClass("hidden");
		$(".depositaddress-no").addClass("hidden");
		$(".depositaddressrequest-yes").addClass("hidden");
		$(".depositaddressrequest-no").addClass("hidden");
		if (xhr.status == 200) {
			var myaccount = JSON.parse(xhr.responseText);

			// Update state of deposit address display
			if (myaccount.DepositAddressValid) {
				$(".depositaddress-yes").removeClass("hidden");
				$(".depositaddress").html(myaccount.DepositAddress);
				var btcURL = "bitcoin:" + myaccount.DepositAddress;
				$("a.depositaddress").attr("href", btcURL);
				// Generate a QR code for the address
				var qrElems = $(".depositaddress-qr").empty().get();
				for (var i = 0; i < qrElems.length; i++) {
					new QRCode(qrElems[i], btcURL);
				}
			} else {
				$(".depositaddress-no").removeClass("hidden");
			}

			// Update state of deposit address request display
			if (myaccount.Account.DepositAddressRequest) {
				$(".depositaddressrequest-yes").removeClass("hidden");
			} else {
				$(".depositaddressrequest-no").removeClass("hidden");
			}

			$(".trustedaccount").html(myaccount.TrustedAccount);
		}
	};
	xhr.open("GET", "/myaccount");
	xhr.setRequestHeader("Accept", "application/json");
	xhr.send();
}
