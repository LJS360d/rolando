import { Schema, model } from 'mongoose';
import { BaseDocument } from './base.document.model';

export interface MarkovChainDocument extends BaseDocument {
	name: string;
	replyRate: number;
	storagePath: string;
}

const MarkovChainSchema = new Schema<MarkovChainDocument>({
	id: { type: String, required: true, unique: true },
	name: { type: String, required: true },
	replyRate: { type: Number, default: 10 },
	storagePath: { type: String, required: true },
});

export const MarkovChainModel = model<MarkovChainDocument>(
	'MarkovChain',
	MarkovChainSchema
);
