import { Model, Document, UpdateQuery, now } from 'mongoose';

export class BaseRepository<T extends Document> {
	public entity: Model<T>;

	async getOne(id: string): Promise<T | null> {
		return await this.entity.findOne({ id }).exec();
	}

	async getAll(): Promise<T[]> {
		return await this.entity.find().exec();
	}

	async create(fields: T): Promise<T> {
		return await this.entity.create({ updatedAt: now(), ...fields });
	}

	async update(id: string, fields: UpdateQuery<T>) {
		return await this.entity
			.updateOne({ id }, { updatedAt: now(), ...fields }, { new: true })
			.exec();
	}

	async delete(id: string) {
		return await this.entity.deleteOne({ id }).exec();
	}
}
