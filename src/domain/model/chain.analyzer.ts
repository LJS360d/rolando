import { formatBytes, formatNumber } from '../../utils/formatting.utils';
import { MarkovChain } from './markov.chain';
import sizeof from 'object-sizeof';

type ChainAnalytics = {
	complexityScore: string;
	gifs: string;
	images: string;
	videos: string;
	replyRate: string;
	words: string;
	messages: string;
	size: string;
};

export class MarkovChainAnalyzer {
	private USE_THRESHOLD = 15;

	constructor(private chain: MarkovChain) {}

	getComplexity(): number {
		const stateSize = Object.keys(this.chain.state).length;
		let highValueWords = 0;

		// O(n) not O(n^2)
		for (const nextWords of Object.values(this.chain.state)) {
			for (const wordValue of Object.values(nextWords)) {
				if (wordValue > this.USE_THRESHOLD) {
					highValueWords++;
				}
			}
		}
		// Calculate the complexity score based on state size and high-value words
		// y = log2(10*x*HVW + 1)
		return Math.ceil(Math.log2(10 * stateSize * highValueWords + 1));
	}

	getAnalytics(): ChainAnalytics {
		return {
			complexityScore: formatNumber(this.getComplexity()),
			gifs: formatNumber(this.chain.mediaStorage.gifs.size),
			images: formatNumber(this.chain.mediaStorage.images.size),
			videos: formatNumber(this.chain.mediaStorage.videos.size),
			replyRate: formatNumber(this.chain.replyRate),
			words: formatNumber(Object.keys(this.chain.state).length),
			messages: formatNumber(this.chain.messageCounter),
			size: formatBytes(sizeof(this.chain)),
		};
	}
}
