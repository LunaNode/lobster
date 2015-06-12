function vmPerform(method, target, params, expect, f, quiet, button) {
	var l = false;

	if(!quiet) {
		$('html').addClass('busy');
		messageClear();

		if(button) {
			l = Ladda.create(button);
			l.start();
		}
	}

	$.get('/panel/csrftoken', function(token) {
		$.ajax({
			method: method,
			url: "/api/vms/" + $("#js_id").html() + target,
			headers: {'Authorization': 'session ' + token},
			data: JSON.stringify(params),
			dataType: expect,
			success: function(data) {
				f(data);

				if(!quiet) {
					$('html').removeClass('busy');
					if(l) l.stop();
				}
			},
			error: function(xhr, textStatus, errorThrown) {
				if(!quiet) {
					if(xhr.responseText) {
						messageUpdate('error', 'failed to complete API call: ' + xhr.responseText);
					} else {
						messageUpdate('error', 'failed to complete API call: ' + textStatus + ' (' + errorThrown + ').');
					}
					$('html').removeClass('busy');
					if(l) l.stop();
				}
			}
		})
	}, 'text')
		.fail(function() {
			if(!quiet) {
				messageUpdate('error', 'failed to complete API call');
				$('html').removeClass('busy');
				if(l) l.stop();
			}
		})
	;
}

function vmAction(action, success_message, update_status, button) {
	vmPerform('POST', '/action', {'action': action}, 'text', function(data) {
		messageUpdate('success', success_message);
		if(update_status) {
			vmStatusUpdate(20);
		}
	}, false, button);
}

function vmStart(button) {
	vmAction('start', 'VM booted', true, button);
}

function vmReboot(button) {
	vmAction('reboot', 'VM rebooted', true, button);
}

function vmStop(button) {
	vmAction('stop', 'VM stopped', true, button);
}

function vmStatusUpdate(ttl) {
	vmPerform('GET', '', {}, 'json', function(data) {
		status = data.details.status;
		statusColor = 'blue';
		if(status == 'Online') {
			statusColor = 'green';
		} else if(status == 'Offline') {
			statusColor = 'red';
		}
		$("#vm_status").html('<font color="' + statusColor + '"><strong>' + data.details.status + '</strong></font>');
	}, true);

	if(ttl && ttl > 0) {
		setTimeout(function() { vmStatusUpdate(ttl - 1); }, 3000);
	}
}

function reloadAddresses() {
	$('#addresses').html('<center><img src="/assets/img/loading.gif"></center>');
	vmPerform('ips', function(data) {
		$.get("/panel/csrftoken", function(token) {
			h = '<table class="table table-striped">\
					<tr>\
						<th>External IP</th>\
						<th>Private IP</th>\
						<th>Action</th>\
					</tr>';
			for(var x in data['ips']) {
				ip = data['ips'][x];
				h += '<tr>';
				if(ip.external_ip) {
					h += '<td>' + ip.external_ip + '</td>';
				} else {
					h += '<td>None</td>';
				}
				if(ip.private_ip) {
					h += '<td>' + ip.private_ip + '</td>';
				} else {
					h += '<td>N/A</td>';
				}
				h += '<td><form method="POST" action="vm.php?tab=vm_ips">';
				h += '<input type="hidden" name="private_ip" value="' + private_ip + '">';
				h += '<input type="hidden" name="floating_ip" value="' + floating_ip + '">';
				h += '<input type="hidden" name="vm_id" value="' + $("#js_id").html() + '">';
				h += '<input type="hidden" name="token" value="' + token + '" />';
				h += '<button type="submit" class="btn btn-danger" name="action" value="delete_ip">Remove</button>';
				if(data['ips'][private_ip]) {
					h += ' <button type="submit" class="btn btn-warning" name="action" value="delete_floatingip">De-associate floating IP</button>';
				} else {
					h += ' <button type="submit" class="btn btn-success" name="action" value="add_floatingip">Associate floating IP</button>';
				}
				h += '</form></td></tr>';
			}
			h += '</table>';
			$('#vm_ips_table').html(h);
		}, 'text');
	}, true);
}
