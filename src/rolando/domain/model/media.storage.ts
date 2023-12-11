import { Firestore } from 'firebase/firestore';
import { getValidUrl, isGifUrl, isImageUrl, isVideoUrl } from '../../utils/url.utils';
type MediaMap = {
	gifs: string[];
	images: string[];
	videos: string[];
};
export class MediaStorage {
	public chainId?: string;
	gifs: Set<string>;
	images: Set<string>;
	videos: Set<string>;
	constructor() {
		this.gifs = new Set<string>();
		this.images = new Set<string>();
		this.videos = new Set<string>();
	}

	get mediaMap(): MediaMap {
		return {
			gifs: Array.from(this.gifs),
			images: Array.from(this.images),
			videos: Array.from(this.videos),
		};
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
				return getValidUrl(this.gifs, type);
			case 'video':
				return getValidUrl(this.videos, type);
			case 'image':
				return getValidUrl(this.images, type);
		}
	}

	removeMedia(url: string): void {
		this.gifs.delete(url);
		this.videos.delete(url);
		this.images.delete(url);
	}

	static from(mediaMap: MediaMap) {
		const mediaStorage = new MediaStorage();
		mediaStorage.gifs = new Set(mediaMap.gifs);
		mediaStorage.images = new Set(mediaMap.images);
		mediaStorage.videos = new Set(mediaMap.videos);
		return mediaStorage;
	}
}
