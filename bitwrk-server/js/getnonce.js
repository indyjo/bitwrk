function getnonce() {
    var xhr = new XMLHttpRequest();
    xhr.open("GET", "/nonce");
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4){
            var nonce = document.getElementById("nonce")
            nonce.value = xhr.responseText
            nonce.readOnly = true
            update()
        }
    };
    xhr.send();
}