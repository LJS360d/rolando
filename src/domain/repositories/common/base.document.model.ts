import { Document } from 'mongoose';

export class BaseDocument extends Document {
	declare id: string;
	updatedAt: NativeDate;
}
