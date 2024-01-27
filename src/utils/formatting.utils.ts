export function formatTime(milliseconds: number) {
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

export function formatBytes(bytes: number): string {
	const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];

	if (bytes === 0) return '0 Byte';

	const i = Math.floor(Math.log2(bytes) / 10);
	const formattedSize = (bytes / Math.pow(1024, i)).toFixed(2);

	return `${formattedSize} ${sizes[i]}`;
}

export function formatNumber(number: number) {
	const parts = number.toString().split('.');
	parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, "'");
	return parts.join('.');
}

export function toLowerSnakeCase(str: string): string {
	return str
		.replace(/([A-Z])/g, '_$1')
		.replace(/^_/g, '')
		.toLowerCase();
}

export function capitalize(str: string): string {
	return str.charAt(0).toUpperCase() + str.slice(1).toLowerCase();
}

export namespace md {
	export function code(str: any): string {
		return `\`${str}\``;
	}
	export function bold(str: any): string {
		return `**${str}**`;
	}
	export function italic(str: any): string {
		return `*${str}*`;
	}
	export function underline(str: any): string {
		return `__${str}__`;
	}
	export function strikethrough(str: any): string {
		return `~~${str}~~`;
	}
	export function spoiler(str: any): string {
		return `||${str}||`;
	}
	export function quote(str: any): string {
		return `> ${str}`;
	}
	export function link(str: any, url: any): string {
		return `[${str}](${url})`;
	}
	export function image(str: any, url: any): string {
		return `![${str}](${url})`;
	}
	export function codeBlock(str: any, lang?: string): string {
		return `\`\`\`${lang}\n${str}\`\`\``;
	}
}
