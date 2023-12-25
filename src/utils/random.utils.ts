/**
 * The `min` and `max` parameters are inclusive.
 * @param min The minimum number to return.
 * @param max The maximum number to return.
 * @returns A random number between the two numbers.
 */
export function getRandom(min: number, max: number): number {
	return Math.floor(Math.random() * (max - min + 1) + min);
}
