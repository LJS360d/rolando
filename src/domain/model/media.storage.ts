import { getValidUrl, isGifUrl, isImageUrl, isVideoUrl } from '../../utils/url.utils';

export class MediaStorage {
	gifs: Set<string>;
	images: Set<string>;
	videos: Set<string>;
	constructor(
		public chainId: string,
		gifs: string[] = [],
		images: string[] = [],
		videos: string[] = []
	) {
		this.gifs = new Set<string>(gifs);
		this.images = new Set<string>(images);
		this.videos = new Set<string>(videos);
	}

	addMedia(url: string) {
		if (isGifUrl(url)) this.gifs.add(url);
		else if (isVideoUrl(url)) this.videos.add(url);
		else if (isImageUrl(url)) this.images.add(url);
		return;
	}

	async getMedia(type: 'gif' | 'video' | 'image'): Promise<string> {
		switch (type) {
			case 'gif':
				return getValidUrl(this.gifs, this.chainId, type);
			case 'video':
				return getValidUrl(this.videos, this.chainId, type);
			case 'image':
				return getValidUrl(this.images, this.chainId, type);
		}
	}

	removeMedia(url: string): void {
		this.gifs.delete(url);
		this.videos.delete(url);
		this.images.delete(url);
	}
}
