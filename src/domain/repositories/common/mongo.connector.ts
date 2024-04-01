import { Logger } from 'fonzi2';
import mongoose, { type Connection } from 'mongoose';

export async function connectMongo(
	uri: string,
	name?: string
): Promise<Connection | undefined> {
	const load = Logger.loading('Connecting to MongoDB...');
	name ??= 'default';
	try {
		await mongoose.connect(uri, {
			dbName: name,
			appName: name,
		});
		const db = mongoose.connection;
		load.success('Connected to MongoDB!');
		return db;
	} catch (error) {
		load.fail('Failed to connect to MongoDB!');
		return;
	}
}
