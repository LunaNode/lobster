var entityMap = {
	"&": "&amp;",
	"<": "&lt;",
	">": "&gt;",
	'"': '&quot;',
	"'": '&#39;',
	"/": '&#x2F;'
};

function escapehtml(string) {
	return String(string).replace(/[&<>"'\/]/g, function (s) {
		return entityMap[s];
	});
}

function messageUpdate(type, msg) {
	typeCaps = type.charAt(0).toUpperCase() + type.slice(1);
	if(type == 'error') type = 'danger';
	$("#message").html('<div class="alert alert-' + escapehtml(type) + '"><strong>' + escapehtml(typeCaps) + ':</strong> ' + escapehtml(msg) + '.</div>');
}

function messageClear() {
	$("#message").html('');
}
