function updateDepositInfoSignature(element) {
    var q = "depositaddress=" + encodeURIComponent(document.getElementById("depositaddress").value);
    q = q + "&nonce=" + encodeURIComponent(document.getElementById("nonce").value);
    q = q + "&participant=" + encodeURIComponent(document.getElementById("participant").value);
    q = q + "&reference=" + encodeURIComponent(document.getElementById("reference").value);
    q = q + "&signer=" + encodeURIComponent(document.getElementById("signer").value);
    element.value = q;
}