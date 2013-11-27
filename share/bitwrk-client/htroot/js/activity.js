function setActivities(node, activitiesjson) {
    var activities = JSON.parse(activitiesjson);
    while (node.hasChildNodes()) {
        node.removeChild(node.lastChild);
    }
    
    for (var i=0; i<activities.length; i++) {
        var activity = activities[i];
        
        var item = document.createElement("div");
        item.setAttribute("class", "activity");
        node.appendChild(item);
        
        var elem = document.createElement("div");
        elem.setAttribute("class", "type");
        elem.innerHTML = activity["Info"]["Type"];
        item.appendChild(elem);

        if (activity["Info"]["Accepted"]) {
            var elem = document.createElement("div");
            elem.setAttribute("class", "price");
            elem.innerHTML = activity["Info"]["Amount"];
            item.appendChild(elem);
        } else {
            // accept button
            var elem = document.createElement("input");
            elem.setAttribute("type", "button");
            elem.setAttribute("class", "closebtn btn btn-primary btn-xs");
            elem.value = "Permit";
            var info = activity["Info"]
            elem.onclick = function(info) {
                return function() {
                    showMandateDialog(info["Type"], info["Article"]);
                };
            }(info);
            item.appendChild(elem);
        
            // close button
            var elem = document.createElement("input");
            elem.setAttribute("type", "button");
            elem.setAttribute("class", "closebtn btn btn-default btn-xs");
            elem.value = "Cancel";
            var key = activity["Key"]
            elem.onclick = function(key) {
                return function() {
                    forbidActivityAsync(key);
                };
            }(key);
            item.appendChild(elem);
        }
        
        var elem = document.createElement("div");
        elem.setAttribute("class", "article");        
        elem.innerHTML = activity["Info"]["Article"];
        item.appendChild(elem);

        var elem = document.createElement("div");
        elem.setAttribute("class", "info");        
        elem.appendChild(document.createTextNode(activity["Info"]["Info"]));
        item.appendChild(elem);
    }
}

function forbidActivityAsync(key) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            updateActivities();
        }
    };
    xhr.open("GET", "/forbid?activity="+key);
    xhr.send();
}

function updateActivities() {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            setActivities(
                document.getElementById("activities"),
                xhr.responseText);
        }
    };
    xhr.open("GET", "/activities");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.send();
}
