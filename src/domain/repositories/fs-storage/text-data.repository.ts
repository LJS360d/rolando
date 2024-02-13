import { Logger } from 'fonzi2';
import {
	appendFileSync,
	closeSync,
	existsSync,
	mkdirSync,
	openSync,
	readFileSync,
	readdirSync,
	writeFileSync,
} from 'fs';
import { join } from 'path';

export class TextDataRepository {
	protected readonly dataFolder = join(process.cwd(), '/data');
	protected readonly fileEncoding = 'utf-8';
	protected readonly fileExtension = 'txt';

	protected ensureDataFolderExists(): void {
		if (!existsSync(this.dataFolder)) {
			mkdirSync(this.dataFolder);
		}
	}

	protected getFilePath(filename: string): string {
		return join(this.dataFolder, `${filename}.${this.fileExtension}`);
	}

	createTextStorage(filename: string, override = false): void {
		this.ensureDataFolderExists();
		const filepath = this.getFilePath(filename);
		void closeSync(openSync(filepath, override ? 'w' : 'a'));
		Logger.info(`Created storage file at ${filepath}`);
	}

	getStorageRefs() {
		this.ensureDataFolderExists();
		const storageRefs: string[] = [];
		const files = readdirSync(this.dataFolder);
		for (const file of files) {
			if (file.endsWith(this.fileExtension)) {
				storageRefs.push(file.replace(`.${this.fileExtension}`, ''));
			}
		}
		return storageRefs;
	}

	getTextData(filename: string): string[] {
		const filepath = this.getFilePath(filename);
		try {
			const fileContent: string = readFileSync(filepath, this.fileEncoding);
			return fileContent.split('\n');
		} catch (err) {
			Logger.warn(`Could not read ${filepath}: ${err}`);
			if (!existsSync(filepath)) {
				this.createTextStorage(filename);
			}
			return [];
		}
	}

	saveTextData(filename: string, text: string | string[]): void {
		const messagesFilepath = this.getFilePath(filename);
		if (!existsSync(messagesFilepath)) {
			this.createTextStorage(filename);
		}
		const contentToAppend = typeof text === 'string' ? `${text}\n` : text.join('\n');
		appendFileSync(messagesFilepath, contentToAppend, this.fileEncoding);
	}

	deleteTextData(filename: string, text: string) {
		const textData = this.getTextData(filename).join('\n');
		const cleanedTextData = textData.replace(new RegExp(text, 'g'), '');
		writeFileSync(this.getFilePath(filename), cleanedTextData, this.fileEncoding);
	}
}
