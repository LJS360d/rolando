import { formatBytes } from '../../utils/formatting.utils';
import { MarkovChain } from './markov.chain';
import sizeof from 'object-sizeof';

type ChainAnalytics = {
	complexityScore: number;
	gifs: number;
	images: number;
	videos: number;
	replyRate: number;
	words: number;
};

export class MarkovChainAnalyzer {
	private USE_THRESHOLD = 15;

	constructor(private chain: MarkovChain) {}

	get size() {
		return formatBytes(sizeof(this.chain));
	}

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
			complexityScore: this.getComplexity(),
			gifs: this.chain.mediaStorage.gifs.size,
			images: this.chain.mediaStorage.images.size,
			videos: this.chain.mediaStorage.videos.size,
			replyRate: this.chain.replyRate,
			words: Object.keys(this.chain.state).length,
		};
	}
}
