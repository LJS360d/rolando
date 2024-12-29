import { now } from 'mongoose';
import type { TextDataRepository } from '../fs-storage/text-data.repository';
import { type ChainDocumentFields, ChainModel } from './models/chain.model';

export class ChainsRepository {
	private readonly entity = ChainModel;
	constructor(private textDataRepository: TextDataRepository) {}

	async getAll() {
		return await this.entity.find();
	}

	async getOne(id: string) {
		return await this.entity.findOne({ id });
	}

	async create(id: string, fields: ChainDocumentFields) {
		this.textDataRepository.createTextStorage(id);
		await this.entity.create({
			id,
			...fields,
			updatedAt: now(),
		});
	}

	async update(id: string, fields: Partial<ChainDocumentFields>) {
		return await this.entity.updateOne({ id }, { ...fields, updatedAt: now() });
	}

	async delete(id: string) {
		await this.entity.deleteOne({ id });
		return;
	}

	updateState(id: string, text: string | string[]): void {
		this.textDataRepository.saveTextData(id, text);
	}

	deleteTextData(id: string, text: string): void {
		this.textDataRepository.deleteTextData(id, text);
	}

	getChainMessages(id: string) {
		return this.textDataRepository.getTextData(id);
	}
}
