function showIframeDialog(url) {
    var dlg = $('#iframeModal');
    dlg.find("#iframe").attr("src", url);
    dlg.modal({'show':true});
}
