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

$(document).ready(function() {
	// action buttons
	$('.lobster-btn').each(function() {
		$(this).click(function() {
			var action = $(this).data('action');
			var token = $(this).data('token');
			var confirmation = $(this).data('confirmation');

			if(confirmation) {
				var result = window.confirm(confirmation);
				if(!result) {
					return;
				}
			}

			var form = $('<form>', {method: 'POST', action: action});
			$('<input>').attr({type: 'hidden', name: 'token', value: token}).appendTo(form);
			$('body').append(form);
			form.submit();
		});
	});
});
