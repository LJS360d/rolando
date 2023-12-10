updateUptime();
setInterval(updateUptime, 1000);
function updateUptime() {
	const uptimeElem = document.getElementById('uptime');
	uptimeElem.textContent = formatTime(
		Date.now() - Number(uptimeElem.getAttribute('start'))
	);
}
function formatTime(milliseconds) {
	const units = [
		{ label: 'y', divisor: 1000 * 60 * 60 * 24 * 30 * 12 },
		{ label: 'm', divisor: 1000 * 60 * 60 * 24 * 30 },
		{ label: 'd', divisor: 1000 * 60 * 60 * 24 },
		{ label: 'h', divisor: 1000 * 60 * 60 },
		{ label: 'm', divisor: 1000 * 60 },
		{ label: 's', divisor: 1000 },
	];

	let output = '';
	for (const unit of units) {
		const value = Math.floor(milliseconds / unit.divisor);
		if (value > 0 || output !== '') {
			output += `${value.toString().padStart(2, '0')}${unit.label} `;
			milliseconds %= unit.divisor;
		}
	}

	return output.trim();
}
