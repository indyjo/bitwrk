function setAccountInfo(node, accountjson) {
    var account = JSON.parse(accountjson);
    
    $(node).find(".available span").text(account["Available"])
    $(node).find(".blocked span").text(account["Blocked"])
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
