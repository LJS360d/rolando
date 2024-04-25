import {
	getValidUrlFromSet,
	isGifUrl,
	isImageUrl,
	isVideoUrl,
} from '../../utils/url.utils';

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

	async getMedia(type: 'gif' | 'video' | 'image'): Promise<string | null> {
		switch (type) {
			case 'gif':
				return getValidUrlFromSet(this.gifs, this.chainId);
			case 'video':
				return getValidUrlFromSet(this.videos, this.chainId);
			case 'image':
				return getValidUrlFromSet(this.images, this.chainId);
		}
	}

	removeMedia(url: string): void {
		this.gifs.delete(url);
		this.videos.delete(url);
		this.images.delete(url);
	}
}
