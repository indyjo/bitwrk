function showMandateDialog(type, articleId) {
    var dlg = $('#mandateModal');
    dlg.find("#perm-type-span").text(type);
    dlg.find("#perm-articleid-span").text(articleId);
    dlg.find("input[name='type']").val(type);
    dlg.find("input[name='articleid']").val(articleId);
    dlg.modal({'show':true});
}
