function updateRequestDepositAddressSignature(element) {
    var q = "nonce=" + encodeURIComponent(document.getElementById("rda_nonce").value);
    q = q + "&participant=" + encodeURIComponent(document.getElementById("rda_participant").value);
    q = q + "&signer=" + encodeURIComponent(document.getElementById("rda_signer").value);
    element.value = q;
}