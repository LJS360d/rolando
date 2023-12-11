import {
	CollectionReference,
	Firestore,
	addDoc,
	collection,
	deleteDoc,
	doc,
	getDocs,
	setDoc,
	updateDoc,
} from 'firebase/firestore';
import { MarkovChain } from '../model/markov.chain';
import { MediaStorage } from '../model/media.storage';
import { Logger } from '../../../fonzi2/lib/logger';

export class ChainsService {
	private readonly firestore: Firestore;
	private readonly chainsCollectionRef: CollectionReference;
	chainsMap: Map<string, MarkovChain>;

	constructor(firestore: Firestore) {
		this.firestore = firestore;
		this.chainsCollectionRef = collection(this.firestore, 'chains');
		this.chainsMap = new Map<string, MarkovChain>();
	}

	async getChain(id: string) {
		const chain = this.chainsMap.get(id);
		if (!chain) {
			return await this.createChain(id);
		}
		return chain;
	}

	async createChain(id: string) {
		Logger.info(`Creating chain ${id}`);
		return await this.setChainDoc(new MarkovChain(id, new MediaStorage()));
	}

	async updateChain(chain: MarkovChain) {
		return await this.updateChainDoc(chain);
	}

	async deleteChain(id: string): Promise<void> {
		const chainDoc = doc(this.firestore, 'chains', id);
		await deleteDoc(chainDoc);

		this.chainsMap.delete(id);
	}

	async loadChains(): Promise<void> {
		const load = Logger.loading();
		load('Started loading Chains...');
		const querySnapshot = await getDocs(this.chainsCollectionRef);

		querySnapshot.forEach((doc) => {
			const { state, media } = doc.data();
			this.chainsMap.set(
				doc.id,
				MarkovChain.from(JSON.parse(state), MediaStorage.from(media))
			);
		});
		load(`Successfully loaded ${this.chainsMap.size} Chains`);
	}

	private async setChainDoc(chain: MarkovChain): Promise<MarkovChain> {
		const chainDoc = doc(this.firestore, 'chains', chain.id);

		await setDoc(chainDoc, {
			state: JSON.stringify(chain.state),
			media: JSON.stringify(chain.mediaStorage.mediaMap),
			reply_rate: chain.replyRate,
		});

		this.chainsMap.set(chain.id, chain);
		return chain;
	}

	private async updateChainDoc(chain: MarkovChain) {
		const chainDoc = doc(this.firestore, 'chains', chain.id);

		await updateDoc(chainDoc, {
			state: JSON.stringify(chain.state),
			media: JSON.stringify(chain.mediaStorage.mediaMap),
			reply_rate: chain.replyRate,
		});

		return chain;
	}
}
