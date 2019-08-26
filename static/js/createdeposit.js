function update() {
	var account = document.getElementById("account").value;
    var q = "account=" + encodeURIComponent(account.replace(/\s+/g, ''));
    
	var amount = document.getElementById("amount").value;
    q = q + "&amount=" + encodeURIComponent(amount.replace(/\s+/g, ''));
    q = q + "&nonce=" + (document.getElementById("nonce").value.replace(/\s+/g, ''));
    q = q + "&ref=" + (document.getElementById("ref").value);
    q = q + "&type=" + (document.getElementById("type").value.replace(/\s+/g, ''));
    q = q + "&uid=" + (document.getElementById("uid").value.replace(/\s+/g, ''));
    document.getElementById("query").value = q;
}