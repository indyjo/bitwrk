(function() {
    var button = document.createElement("button");
    button.type="button";
    button.innerHTML="Show JSON";
    button.onclick=function() {
        var textbox = document.createElement("textarea");
        textbox.setAttribute("rows", 10);
        textbox.setAttribute("cols", 60);
        if (button.nextSibling)
            document.body.insertBefore(textbox, button.nextSibling);
        else
            document.body.appendChild(textbox);
        
        document.body.removeChild(button)
        
        var xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState === 4){
                textbox.value=JSON.stringify(JSON.parse(xhr.responseText), undefined, 2);
            }
        };
        xhr.open("GET", document.location);
        xhr.setRequestHeader("Accept", "application/json");
        xhr.send();
    }
    document.body.appendChild(button);
}) ()