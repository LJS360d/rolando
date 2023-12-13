import axios from 'axios';

function extractUrlInfo(url: string): { domain?: string; extension?: string } {
	const extension = getUrlExtension(url);
	const domain = getUrlDomain(url);
	return { domain, extension };
}

export function containsURL(text: string): boolean {
	const urlRegex = /(https?|ftp):\/\/[^\s/$.?#].[^\s]*/;
	const matches = urlRegex.exec(text);

	if (matches) {
		for (const url of matches) {
			try {
				const { protocol } = new URL(url);
				if (protocol === 'http:' || protocol === 'https:') {
					return true;
				}
			} catch (error) {
				// Ignore invalid URLs
			}
		}
	}

	return false;
}

export function getUrlExtension(url: string) {
	try {
		return new URL(url).pathname.match(/\.[^./?]+(?=\?|$| )/)?.[0];
	} catch (error) {
		// Invalid URL
		return undefined;
	}
}

export function getUrlDomain(url: string) {
	try {
		return new URL(url).hostname;
	} catch (error) {
		// Invalid URL
		return undefined;
	}
}

export async function validateUrl(url: string): Promise<boolean> {
	try {
		const response = await axios.head(url);
		return response.status === 200;
	} catch (error) {
		return false;
	}
}

export function isGifUrl(url: string) {
	const { domain, extension } = extractUrlInfo(url);
	const supportedExtensions = ['.gif'];
	const supportedDomains = ['tenor.com', 'giphy.com'];
	if (domain && extension)
		return supportedExtensions.includes(extension) || supportedDomains.includes(domain);
	else return false;
}

export function isImageUrl(url: string) {
	const { domain, extension } = extractUrlInfo(url);
	const supportedExtensions = ['.png', '.jpg', '.jpeg', '.webp'];
	const supportedDomains = ['imgur.com'];
	if (domain && extension)
		return supportedExtensions.includes(extension) || supportedDomains.includes(domain);
	else return false;
}

export function isVideoUrl(url: string) {
	const { domain, extension } = extractUrlInfo(url);
	const supportedExtensions = ['.mp4', '.mov'];
	const supportedDomains = ['www.youtube.com', 'youtu.be'];
	if (domain && extension)
		return supportedExtensions.includes(extension) || supportedDomains.includes(domain);
	else return false;
}

export async function getValidUrl(urlsSet: Set<string>, type?: string): Promise<string> {
	const urls = Array.from(urlsSet);
	while (urls.length > 0) {
		const randomIndex = Math.floor(Math.random() * urls.length);
		const media = urls[randomIndex];

		if (await validateUrl(media)) {
			// Valid URL
			return media;
		}

		// Remove invalid URL from set
		urlsSet.delete(media);
	}

	return `No valid ${type ?? 'URL'} found`;
}
