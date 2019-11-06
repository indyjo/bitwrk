function update() {
    var enabled = document.getElementById("enabled").value.replace(/\s+/g, '');
    var nonce = document.getElementById("nonce").value.replace(/\s+/g, '');
    var source = document.getElementById("source").value.replace(/\s+/g, '');
    var target = document.getElementById("target").value.replace(/\s+/g, '');
    var type = document.getElementById("type").value.replace(/\s+/g, '');

    var q = "enabled=" + encodeURIComponent(enabled);
    q = q + "&nonce=" + encodeURIComponent(nonce);
    q = q + "&source=" + encodeURIComponent(source);
    q = q + "&target=" + encodeURIComponent(target);
    q = q + "&type=" + encodeURIComponent(type);
    document.getElementById("query").value = q;
}