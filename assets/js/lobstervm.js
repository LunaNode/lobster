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
		if(params) {
			params = JSON.stringify(params);
		}
		$.ajax({
			method: method,
			url: "/api/vms/" + $("#js_id").html() + target,
			headers: {'Authorization': 'session ' + token},
			data: params,
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
						messageUpdate('error', xhr.responseText);
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
		$("#vm_status").html('<font color="' + escapehtml(statusColor) + '"><strong>' + escapehtml(data.details.status) + '</strong></font>');
	}, true);

	if(ttl && ttl > 0) {
		setTimeout(function() { vmStatusUpdate(ttl - 1); }, 3000);
	}
}

function reloadAddresses() {
	$('#vm_addresses_table').html('<center><img src="/assets/img/loading.gif"></center>');
	vmPerform('GET', '/ips', null, 'json', function(data) {
		if(data['addresses']) {
			showPrivate = data['addresses'][0].private_ip;
			showHostname = data['addresses'][0].hostname != '.' && data['addresses'][0].can_rdns;

			h = '<table class="table table-striped">';
			h += '<tr>';
			h += '<th>External IP</th>';
			if(showPrivate) h += '<th>Private IP</th>';
			if(showHostname) h += '<th>Hostname</th>';
			h += '<th>Action</th>';
			h += '</tr>';

			for(var x in data['addresses']) {
				ip = data['addresses'][x];
				h += '<tr>';
				if(ip.ip) {
					h += '<td>' + escapehtml(ip.ip) + '</td>';
				} else {
					h += '<td>None</td>';
				}
				if(showPrivate) {
					if(ip.private_ip) {
						h += '<td>' + escapehtml(ip.private_ip) + '</td>';
					} else {
						h += '<td>N/A</td>';
					}
				}
				if(showHostname) h += '<td>' + escapehtml(ip.hostname) + '</td>';
				h += '<td>';
				if(ip.can_rdns) {
					h += ' <button type="button" class="btn btn-primary" onclick="showSetRdns(\'' + escapehtml(ip.ip) + '\', \'' + escapehtml(ip.hostname) + '\');">Set rDNS</button>';
				}
				h += ' <button type="button" class="btn btn-danger ladda-button" data-style="expand-right" data-size="l" onclick="addressRemove(this, \'' + escapehtml(ip.ip) + '\', \'' + escapehtml(ip.private_ip) + '\');">Remove</button>';
				h += '</td>';
				h += '</tr>';
			}
			h += '</table>';
			$('#vm_addresses_table').html(h);
		} else {
			$('#vm_addresses_table').html('<strong>This VM does not have any IP addresses.</strong>');
		}
	}, true);
}

function addressAdd(button) {
	vmPerform('POST', '/ips/add', null, 'text', reloadAddresses, false, button);
}

function addressRemove(button, ip, privateIp) {
	vmPerform('POST', '/ips/remove', {'ip': ip, 'private_ip': privateIp}, 'text', reloadAddresses, false, button);
}

function showSetRdns(ip, hostname) {
	$('#modal_rdns_ip').text(ip);
	$('#modal_rdns_hostname').val(hostname);
	$('#modal_rdns_form').attr('action', '/panel/vms/' + $("#js_id").html() + '/ips/' + escapehtml(ip) + '/rdns');
	$('#modalRdns').modal('show');
}

function setRdns(button) {
	vmPerform('POST', '/ips/' + $('#modal_rdns_ip').html() + '/rdns', {'hostname': $('#modal_rdns_hostname').val()}, 'text', function(){}, false, button);
	$('#modalRdns').modal('hide');
	reloadAddresses();
}

$('a[data-toggle="tab"]').on('show.bs.tab', function (e) {
	if((e.target + "").split("#")[1] == 'vm_addresses') {
		reloadAddresses();
	}
});
