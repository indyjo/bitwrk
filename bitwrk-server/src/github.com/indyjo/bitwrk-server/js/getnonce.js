function getNonceFor(element) {
    var xhr = new XMLHttpRequest();
    xhr.open("GET", "/nonce");
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4){
            element.value = xhr.responseText
            element.readOnly = true
            update()
        }
    };
    xhr.send();
}
function getnonce() {
    getNonceFor(document.getElementById("nonce"))
}