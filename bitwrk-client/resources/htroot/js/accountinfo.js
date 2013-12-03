function setAccountInfo(node, accountjson) {
    var account = JSON.parse(accountjson);
    while (node.hasChildNodes()) {
        node.removeChild(node.lastChild);
    }
    
    var item = document.createElement("div");
    item.setAttribute("class", "participant");
    item.innerHTML = account["Participant"];
    node.appendChild(item);
    
    var item = document.createElement("div");
    item.setAttribute("class", "available");
    item.innerHTML = account["Available"];
    node.appendChild(item);
}

function updateAccountInfoFor(participantId) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            setAccountInfo(
                document.getElementById("account-info"),
                xhr.responseText);
        }
    };
    xhr.open("GET", "/account/"+participantId);
    xhr.setRequestHeader("Accept", "application/json");
    xhr.send();
}
