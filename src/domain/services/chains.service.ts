import { Logger } from 'fonzi2';
import { MarkovChain } from '../model/markov.chain';
import { ChainsRepository } from '../repositories/chains/chains.repository';
import { Client } from 'discord.js';
import { ChainDocumentFields } from '../repositories/chains/models/chain.model';
import { Container } from 'typedi';
import sizeof from 'object-sizeof';
import { formatBytes } from '../../utils/formatting.utils';

export class ChainsService {
	private readonly chainsMap: Map<string, MarkovChain>;

	constructor(private chainsRepository: ChainsRepository) {
		this.chainsMap = new Map<string, MarkovChain>();
	}

	get chains() {
		return this.chainsMap;
	}

	async getChain(id: string): Promise<MarkovChain> {
		const chain = this.chainsMap.get(id);
		if (!chain) {
			const guild = await Container.get(Client).guilds.fetch(id);
			return await this.createChain(id, guild.name);
		}
		return chain;
	}

	async getChainDocument(id: string) {
		return await this.chainsRepository.getOne(id);
	}

	async createChain(id: string, name: string): Promise<MarkovChain> {
		Logger.info(`Creating chain ${name}`);
		const chain = new MarkovChain(id);
		this.chainsMap.set(id, chain);
		await this.chainsRepository.create(id, { name });
		return chain;
	}

	async updateChainState(id: string, text: string | string[]): Promise<MarkovChain> {
		const chain = await this.getChain(id);
		if (typeof text === 'string') {
			chain.updateState(text);
			this.chainsRepository.updateState(chain.id, text);
			return chain;
		}
		chain.provideData(text);
		this.chainsRepository.updateState(chain.id, text);
		return chain;
	}

	async updateChainProps(id: string, fields: Partial<ChainDocumentFields>) {
		return this.chainsRepository.update(id, fields);
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
			const messages = this.getChainMessages(chain.id);

			this.chainsMap.set(chain.id, new MarkovChain(chain.id, chain.replyRate, messages));
		}
		load.success(`Loaded ${this.chainsMap.size} Chains`);
		Logger.info(`Chains total size: #green${this.getChainsMemUsage()}$`);
	}

	getChainMessages(id: string) {
		return this.chainsRepository.getChainMessages(id);
	}

	getChainsMemUsage() {
		const chainsSize = Array.from(this.chainsMap.values())
			.map((chain) => sizeof(chain))
			.reduce((a, b) => a + b, 0);
		return formatBytes(chainsSize);
	}
}
