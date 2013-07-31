function append(str, id) {
    var e = document.getElementById(id);
    if (e == null) return str;
    var v = e.value.replace(/\s+/g, '');
    if (v == "") return str;
    str = str + (str==""?"":"&") + id + "=" + encodeURIComponent(v);
    return str;
}

function appendCheck(str, id) {
    var e = document.getElementById(id);
    if (e == null || !e.checked) return str;
    str = str + (str==""?"":"&") + id + "=on";
    return str;
}

function update() {
    var q = "";
    // Arguments must appear in alphabetical order
    q = appendCheck(q, "acceptresult");
    q = append(q, "buyersecret");
    q = append(q, "encresulthash");
    q = append(q, "encresulthashsig");
    q = append(q, "encresultkey");
    q = appendCheck(q, "rejectresult");
    q = appendCheck(q, "rejectwork");
    q = append(q, "txid");
    q = append(q, "workerurl");
    q = append(q, "workhash");
    q = append(q, "worksecrethash");
    document.getElementById("query").value = q;
}
