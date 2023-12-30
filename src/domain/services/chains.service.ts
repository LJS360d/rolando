import axios from 'axios';
import {
	Firestore,
	collection,
	deleteDoc,
	doc,
	getDocs,
	setDoc,
	updateDoc,
} from 'firebase/firestore';
import { FirebaseStorage, getDownloadURL, ref, uploadBytes } from 'firebase/storage';
import { MarkovChain } from '../model/markov.chain';
import { Logger } from 'fonzi2';

export class ChainsService {
	private readonly chainsMap: Map<string, MarkovChain>;

	constructor(
		private readonly firestore: Firestore,
		private readonly storage: FirebaseStorage
	) {
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
		return await this.setChainDoc(new MarkovChain(id, name));
	}

	async updateChain(chain: MarkovChain, text: string | string[]): Promise<MarkovChain> {
		if (typeof text === 'string') {
			chain.updateState(text);
			return await this.updateChainDoc(chain, text);
		}
		chain.provideData(text);
		return await this.updateChainDoc(chain, text.join('\n'));
	}

	async updateChainProps(chain: MarkovChain) {
		const chainDocRef = doc(this.firestore, 'chains', chain.id);
		await updateDoc(chainDocRef, {
			reply_rate: chain.replyRate,
			name: chain.name,
		});
		return chain;
	}

	async deleteChain(id: string): Promise<void> {
		Logger.warn(`Deleting chain ${id}`);
		const chainDoc = doc(this.firestore, 'chains', id);
		await deleteDoc(chainDoc);
		this.chainsMap.delete(id);
	}

	async loadChains(): Promise<void> {
		const load = Logger.loading('Loading Chains...');
		const querySnapshot = await getDocs(collection(this.firestore, 'chains'));
		const messagePromises = querySnapshot.docs.map(async (doc) => {
			const { messages, name, reply_rate } = doc.data();
			const messagesText = (await axios.get<string>(messages)).data.split('\n');
			this.chainsMap.set(doc.id, new MarkovChain(doc.id, name, reply_rate, messagesText));
		});

		await Promise.all(messagePromises);

		load.success(`Successfully loaded ${this.chainsMap.size} Chains`);
		this.chainsMap.forEach((chain) => {
			Logger.info(`Chain ${chain.name} size: #green${chain.size}$`);
		});
	}

	private async setChainDoc(chain: MarkovChain): Promise<MarkovChain> {
    const chainDocRef = doc(this.firestore, 'chains', chain.id);
		const messagesRef = ref(this.storage, `messages/${chain.id}.txt`);
		const blob = new Blob([], { type: 'text/plain' });
		await uploadBytes(messagesRef, blob);
		const messagesUrl = await getDownloadURL(messagesRef);

		await setDoc(chainDocRef, {
			messages: messagesUrl,
			reply_rate: chain.replyRate,
			name: chain.name,
		});

		this.chainsMap.set(chain.id, chain);
		return chain;
	}

	public async deleteChainDocLine(id: string, textToRemove: string) {
		const textFileRef = ref(this.storage, `messages/${id}.txt`);
		const textFileUrl = await getDownloadURL(textFileRef);
		const rawText = (await axios.get<string>(textFileUrl)).data;
		const newContent = rawText.replace(new RegExp(textToRemove, 'g'), '');
		const blob = new Blob([newContent], { type: 'text/plain' });
		await uploadBytes(textFileRef, blob);
		return textFileUrl;
	}

	private async updateChainDoc(chain: MarkovChain, newText: string) {
		const textFileUrl = await this.updateGuildTextFileContent(chain.id, newText);
		const chainDocRef = doc(this.firestore, 'chains', chain.id);
		await updateDoc(chainDocRef, {
			messages: textFileUrl,
			reply_rate: chain.replyRate,
			name: chain.name,
		});
		return chain;
	}

	private async updateGuildTextFileContent(id: string, newText: string) {
		const textFileRef = ref(this.storage, `messages/${id}.txt`);
		const textFileUrl = await getDownloadURL(textFileRef);
		const rawText = (await axios.get<string>(textFileUrl)).data;
		const newContent = `${rawText}\n${newText}`;
		const blob = new Blob([newContent], { type: 'text/plain' });
		await uploadBytes(textFileRef, blob);
		return textFileUrl;
	}
}
