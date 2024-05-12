/**
 * The `min` and `max` parameters are inclusive.
 * @param min The minimum number to return.
 * @param max The maximum number to return.
 * @returns A random number between the two numbers.
 */
export function getRandom(min: number, max: number): number {
	return Math.floor(Math.random() * (max - min + 1) + min);
}

export function getMessagesPerSecond(
	messagesTotal: number,
	milliseconds: number
) {
	return Math.round((messagesTotal * 1000) / milliseconds);
}

export function getMessagesPerMinute(
	messagesTotal: number,
	milliseconds: number
) {
	return getMessagesPerSecond(messagesTotal, milliseconds) * 60;
}

export function getStatIncrement(prev: number, curr: number) {
	const diff = curr - prev;
	return `${prev} -> ${curr} (${diff >= 0 ? '+' : ''}${diff})`;
}
