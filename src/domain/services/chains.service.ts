import { Logger } from 'fonzi2';
import { MarkovChain } from '../model/markov.chain';
import { ChainsRepository } from '../repositories/chains.repository';

export class ChainsService {
	private readonly chainsMap: Map<string, MarkovChain>;

	constructor(private chainsRepository: ChainsRepository) {
		this.chainsMap = new Map<string, MarkovChain>();
	}

	get chains() {
		return this.chainsMap;
	}

	async getChain(id: string, name: string): Promise<MarkovChain> {
		const chain = this.chainsMap.get(id);
		if (!chain) {
			return await this.createChain(id, name);
		}
		return chain;
	}

	async createChain(id: string, name: string): Promise<MarkovChain> {
		Logger.info(`Creating chain ${name}`);
		const chain = new MarkovChain(id, name);
		this.chainsMap.set(id, chain);
		await this.chainsRepository.create(chain);
		return chain;
	}

	async updateChain(chain: MarkovChain, text: string | string[]): Promise<MarkovChain> {
		if (typeof text === 'string') {
			chain.updateState(text);
			this.chainsRepository.updateState(chain, text);
			return chain;
		}
		chain.provideData(text);
		this.chainsRepository.updateState(chain, text);
		return chain;
	}

	async updateChainProps(chain: MarkovChain) {
		this.chainsRepository.updateCommon(chain);
		return chain;
	}

	async deleteChain(id: string): Promise<void> {
		Logger.warn(`Deleting chain ${id}`);
		this.chainsRepository.delete(id);
		this.chainsMap.delete(id);
	}

	async loadChains(): Promise<void> {
		const load = Logger.loading('Loading Chains...');
		const chains = await this.chainsRepository.getAll();
		for (const chain of chains) {
			const messages = this.chainsRepository.getChainMessages(chain.id);

			this.chainsMap.set(
				chain.id,
				new MarkovChain(chain.id, chain.name, chain.replyRate, messages)
			);
		}

		load.success(`Loaded ${this.chainsMap.size} Chains`);
		this.chainsMap.forEach((chain) => {
			Logger.info(`Chain ${chain.name} size: #green${chain.size}$`);
		});
	}
}
