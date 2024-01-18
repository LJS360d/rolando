import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'fs';
import { now } from 'mongoose';
import { join } from 'path';
import { MarkovChain } from '../model/markov.chain';
import { MarkovChainModel } from './models/markov.chain.model';
import { Logger } from 'fonzi2';

export class ChainsRepository {
	private dataFolder = join(process.cwd(), 'messages');
	private readonly fileEncoding = 'utf-8';
	constructor() {}

	async getAll() {
		return await MarkovChainModel.find();
	}

	async getOne(id: string) {
		return await MarkovChainModel.findOne({ id });
	}

	async create(chain: MarkovChain) {
		if (!existsSync(this.dataFolder)) mkdirSync(this.dataFolder);
		const storagePath = join(this.dataFolder, `${chain.id}.txt`);
		writeFileSync(storagePath, '', this.fileEncoding);

		await MarkovChainModel.create({
			id: chain.id,
			replyRate: chain.replyRate,
			storagePath,
			name: chain.name,
			updatedAt: now(),
		});
	}

	async updateCommon(chain: MarkovChain) {
		await MarkovChainModel.updateOne(
			{ id: chain.id },
			{ replyRate: chain.replyRate, name: chain.name, updatedAt: now() }
		);
		return chain;
	}

	updateState(chain: MarkovChain, text: string | string[]): void {
		const messagesFilepath = join(this.dataFolder, `${chain.id}.txt`);
		const fileContent: string = readFileSync(messagesFilepath, this.fileEncoding);
		if (typeof text === 'string') {
			writeFileSync(messagesFilepath, `${fileContent}\n${text}`, this.fileEncoding);
			return;
		}
		writeFileSync(
			messagesFilepath,
			`${fileContent}\n${text.join('\n')}`,
			this.fileEncoding
		);
	}

	getChainMessages(id: string) {
		const messageFilename = `${id}.txt`;
		const messagesFilepath = join(this.dataFolder, messageFilename);
		try {
			const fileContent: string = readFileSync(messagesFilepath, this.fileEncoding);
			return fileContent.split('\n');
		} catch (_) {
			Logger.warn(`Could not read file ${messageFilename}`);
			if (!existsSync(messagesFilepath)) {
				writeFileSync(messagesFilepath, '', this.fileEncoding);
				Logger.info(`Created storage file ${messageFilename}`);
			}
			return [];
		}
	}

	async delete(id: string) {
		await MarkovChainModel.deleteOne({ id });
		return;
	}
}
