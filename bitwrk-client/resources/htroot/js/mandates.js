function setMandates(node, mandatesjson) {
    var mandates = JSON.parse(mandatesjson);
    // build a dictionary of existing nodes
    var itemNodesByKey = {}
    for (var i=0; i<node.childNodes.length; i++) {
        var item = node.childNodes[i];
        var key = item.Key
        if (key) {
            itemNodesByKey[key] = item
        }
    }
    
    // iterate over mandates list
    for (var i=0; i<mandates.length; i++) {
        var mandate = mandates[i];
        var key = mandate.Key;
        var info = mandate.Info;
        
        var item = itemNodesByKey[key];
        delete itemNodesByKey[key];
        
        var needsCreate = true;
        var needsUpdate = true;
        if (item) {
            // Item existed already. Check for equality.
            needsCreate = false;
            var info2 = item.Info;
            if (info.TradesLeft === info2.TradesLeft) {
            	needsUpdate = false
            }
        }
        
        if (needsCreate) {
            // Item is new -> append div to parent node
            item = document.createElement("div");
            item.setAttribute("class", "mandate");
            var html = '<div class="type"></div>' +
				'<input type="button" class="closebtn btn btn-default btn-xs" value="Revoke"></input>' +
				'<div class="price"></div>' +
				'<div class="article"></div>';
			if (info.UseTradesLeft) {
				html += '<div class="tradesleft"></div>';
			}
			if (info.UseUntil) {
				html += '<div class="validuntil"></div>';
			}
			
			item.innerHTML = html;
            node.appendChild(item);
            // Update key attribute
            item.Key = key;
        }
        
        if (needsUpdate) {
			var childIdx = 0;
			item.childNodes[childIdx++].textContent = info.Type === 0 ? "BUY" : "SELL";
			item.childNodes[childIdx++].onclick = function(key) {
				return function() {
					revokeMandateAsync(key);
				};
			}(key);
			item.childNodes[childIdx++].textContent = info.Price;
			item.childNodes[childIdx++].textContent = info.Article;
			if (info.UseTradesLeft) {
				item.childNodes[childIdx++].textContent = "Trades left: " + info.TradesLeft;
			}
			if (info.UseUntil) {
				var millis = Date.parse(info.Until) - (new Date()).getTime();
				item.childNodes[childIdx++].textContent = "Minutes left: " +
					Math.max(0, Math.ceil(millis / 60000));
			}
			item.Info = info;
        }
    }
    
    // delete removed nodes
    for (var key in itemNodesByKey) {
        if (itemNodesByKey.hasOwnProperty(key)) {
            node.removeChild(itemNodesByKey[key]);
        }
    }
}

function revokeMandateAsync(key) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            updateMandates();
        }
    };
    xhr.open("GET", "/revokemandate?key="+key);
    xhr.send();
}

function updateMandates() {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            setMandates(
                document.getElementById("mandates"),
                xhr.responseText);
        }
    };
    xhr.open("GET", "/mandates");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.send();
}
