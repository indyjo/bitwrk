function setWorkers(node, workersjson) {
    var workers = JSON.parse(workersjson);
    // build a dictionary of existing nodes
    var itemNodesByKey = {}
    for (var i=0; i<node.childNodes.length; i++) {
        var item = node.childNodes[i];
        var key = item.Key
        if (key) {
            itemNodesByKey[key] = item
        }
    }
    
    // iterate over workers list
    for (var i=0; i<workers.length; i++) {
        var worker = workers[i];
        var key = worker.Info.Id;
        var info = worker.Info;
        
        var item = itemNodesByKey[key];
        delete itemNodesByKey[key];
        
        var needsCreate = true;
        var needsUpdate = true;
        if (item) {
            // Item existed already. Check for equality.
            needsCreate = false;
            var info2 = item.Info;
            if (info.Article === info2.Article
            	&& info.Method === info2.Method
            	&& info.PushURL === info2.PushURL) {
            	needsUpdate = false
            }
        }
        
        if (needsCreate) {
            // Item is new -> append div to parent node
            item = document.createElement("div");
            item.setAttribute("class", "worker");
			item.innerHTML =
				'<div class="key"></div>' +
				'<input type="button" class="closebtn btn btn-default btn-xs" value="Stop"></input>' +
				'<div class="article"></div>';
            node.appendChild(item);
            // Update key attribute
            item.Key = key;
        }
        
        if (needsUpdate) {
			var childIdx = 0;
			item.childNodes[childIdx++].textContent = info.Id;
			item.childNodes[childIdx++].onclick = function(key) {
				return function() {
					unregisterWorkerAsync(key);
				};
			}(key);
			item.childNodes[childIdx++].textContent = info.Article;
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

function unregisterWorkerAsync(key) {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            updateActivities();
        }
    };
    xhr.open("GET", "/unregisterworker?id="+encodeURIComponent(key));
    xhr.send();
}

function updateWorkers() {
    var xhr = new XMLHttpRequest();
    xhr.onreadystatechange = function() {
        if (xhr.readyState === 4 && xhr.status == 200 ){
            setWorkers(
                document.getElementById("workers"),
                xhr.responseText);
        }
    };
    xhr.open("GET", "/workers");
    xhr.setRequestHeader("Accept", "application/json");
    xhr.send();
}
