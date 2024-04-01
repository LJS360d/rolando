import { Schema, model, now } from 'mongoose';
import type { BaseDocument } from '../../common/base.document.model';

export interface ChainDocumentFields {
	name: string;
	replyRate?: number;
	trained?: boolean;
}

export type ChainDocument = ChainDocumentFields & BaseDocument;

const ChainSchema = new Schema<ChainDocument>({
	id: { type: String, required: true, unique: true },
	name: { type: String, required: true },
	replyRate: { type: Number, default: 10 },
	trained: { type: Boolean, default: false },
	updatedAt: { type: Date, default: now() },
});

export const ChainModel = model<ChainDocument>('chain', ChainSchema);
