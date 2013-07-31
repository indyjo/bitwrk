function update() {
    var q = "article=" + encodeURIComponent(document.getElementById("article").value);
    q = q + "&type=" + (document.getElementById("typebuy").checked?"BUY":"SELL");
    var price = document.getElementById("price").value;
    q = q + "&price=" + encodeURIComponent(price.replace(/\s+/g, ''));
    var address = document.getElementById("address").value;
    q = q + "&address=" + encodeURIComponent(address.replace(/\s+/g, ''));
    var nonce = document.getElementById("nonce").value;
    q = q + "&nonce=" + encodeURIComponent(nonce.replace(/\s+/g, ''));
    document.getElementById("query").value = q;
}