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
            	|| info.Rejected !== info2.Rejected
            	|| info.Alive !== info2.Alive
            	|| info.Rejected !== info2.Rejected
                || info.Article !== info2.Article
                || info.TxId !== info2.TxId
                || info.BidId !== info2.BidId)
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
            node.insertBefore(item, node.firstChild);
            // Update key attribute
            item.Key = key;
        }
        
        
        if (needsFill) {
            // Item is either new or has been emptied -> create children
            if (info.Accepted || !info.Alive) {
                item.innerHTML =
                    '<div class="type"></div>' +
                    '<div class="price"></div>' +
                    '<div class="article"></div>' +
                    (info.BidId?' \u00bb <a>Bid</a>':'') +
                    (info.TxId?' \u00bb <a>Tx</a>':'') +
                    '<div class="info"></div>';
            } else {
                item.innerHTML =
                    '<div class="type"></div>' +
                    '<button class="closebtn btn btn-primary btn-xs">Permit</button>' +
                    '<div class="article"></div>' +
                    '<div class="info"></div>';
            }
            item.setAttribute("class", info.Alive?"activity":"activity history");
            // Update info attribute
            item.Info = info;
        }
        
        var childIdx = 0;
        item.childNodes[childIdx++].textContent = info.Type;
        if (info.Accepted || info.Rejected) {
            item.childNodes[childIdx++].textContent = info.Amount;
            item.childNodes[childIdx++].textContent = info.Article;
            if (info.BidId) {
            	childIdx++; // Skip text
            	var url="http://" + document.location.host + "/bid/" + info.BidId
                item.childNodes[childIdx++].setAttribute("onclick", "showIframeDialog('" + url + "')");
            }
            if (info.TxId) {
            	childIdx++; // Skip text
            	var url="http://" + document.location.host + "/tx/" + info.TxId
                item.childNodes[childIdx++].setAttribute("onclick", "showIframeDialog('" + url + "')");
            }
        } else {
            item.childNodes[childIdx++].onclick = function(info) {
                return function() {
                    showMandateDialog(info.Type, info.Article);
                };
            }(info);
            item.childNodes[childIdx++].textContent = info.Article;
        }
        item.childNodes[childIdx++].textContent = info.Info;
    }
    
    // delete removed nodes
    for (var key in itemNodesByKey) {
        if (itemNodesByKey.hasOwnProperty(key)) {
            node.removeChild(itemNodesByKey[key]);
        }
    }
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
