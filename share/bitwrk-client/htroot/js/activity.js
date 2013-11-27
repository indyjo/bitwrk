function setActivities(node, activitiesjson) {
    var activities = JSON.parse(activitiesjson);
//     while (node.hasChildNodes()) {
//         node.removeChild(node.lastChild);
//     }

    // build a dictionary of existing nodes
    var itemNodesByKey = {}
    for (var i=0; i<node.childNodes.length; i++) {
        var item = node.childNodes[i];
        var key = item.Key
        if (key) {
            itemNodesByKey[key] = item
        }
    }
    
    // iterate over activity list
    for (var i=0; i<activities.length; i++) {
        var activity = activities[i];
        var key = activity.Key;
        var info = activity.Info;
        
        var item = itemNodesByKey[key];
        delete itemNodesByKey[key];
        
        var needsCreate = true;
        var needsFill = true;
        if (item) {
            // Item existed already. Check for equality.
            needsCreate = false;
            var info2 = item.Info;
            if (info.Accepted !== info2.Accepted
                || info.Article !== info2.Article)
            {
                // structural change -> refill
                while (item.hasChildNodes()) {
                    item.removeChild(item.lastChild);
                }
            } else if (info.Amount !== info2.Amount
                || info.Info !== info2.Info)
            {
                // content change -> just update
                needsFill = false;
            } else {
                // no change at all
                continue
            }
            
        }
        
        if (needsCreate) {
            // Item is new -> append div to parent node
            item = document.createElement("div");
            item.setAttribute("class", "activity");
            node.appendChild(item);
            // Update key attribute
            item.Key = key;
        }
        
        if (needsFill) {
            // Item is either new or has been emptied -> create children
            if (info.Accepted) {
                item.innerHTML =
                    '<div class="type"></div>' +
                    '<div class="price"></div>' +
                    '<div class="article"></div>' +
                    '<div class="info"></div>';
            } else {
                item.innerHTML =
                    '<div class="type"></div>' +
                    '<input type="button" class="closebtn btn btn-primary btn-xs" value="Permit"></input>' +
                    '<input type="button" class="closebtn btn btn-default btn-xs" value="Cancel"></input>' +
                    '<div class="article"></div>' +
                    '<div class="info"></div>';
            }
            // Update info attribute
            item.Info = info;
        }
        
        var childIdx = 0;
        item.childNodes[childIdx++].textContent = info.Type;
        if (info.Accepted) {
            item.childNodes[childIdx++].textContent = info.Amount;
        } else {
            item.childNodes[childIdx++].onclick = function(info) {
                return function() {
                    showMandateDialog(info.Type, info.Article);
                };
            }(info);
            item.childNodes[childIdx++].onclick = function(key) {
                return function() {
                    forbidActivityAsync(key);
                };
            }(key);
        }
        item.childNodes[childIdx++].textContent = info.Article;
        item.childNodes[childIdx++].textContent = info.Info;
    }
    
    // delete removed nodes
    for (var key in itemNodesByKey) {
        if (itemNodesByKey.hasOwnProperty(key)) {
            node.removeChild(itemNodesByKey[key]);
        }
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
