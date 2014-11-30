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
		if (xhr.readyState === 4 && xhr.status == 200) {
			var myaccount = JSON.parse(xhr.responseText);
			if (myaccount.DepositAddressValid) {
				$(".depositaddress-yes").removeClass("hidden")
				$(".depositaddress-no").addClass("hidden")
				$(".depositaddress").html(myaccount.DepositAddress)
				$("a.depositaddress").attr("href",
						"bitcoin:" + myaccount.DepositAddress)
			} else {
				$(".depositaddress-yes").addClass("hidden")
				$(".depositaddress-no").removeClass("hidden")
			}
			$(".trustedaccount").html(myaccount.TrustedAccount)
		} else if (xhr.readyState === 4) {
			$(".depositaddress-yes").addClass("hidden")
			$(".depositaddress-no").removeClass("hidden")
		}
	};
	xhr.open("GET", "/myaccount");
	xhr.setRequestHeader("Accept", "application/json");
	xhr.send();
}
