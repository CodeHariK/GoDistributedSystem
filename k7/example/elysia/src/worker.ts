import { readFile, readdir } from "fs/promises";
import { join } from "path";

// CPU-heavy task: Compute Fibonacci
const fib = (n: number): number => (n <= 1 ? n : fib(n - 1) + fib(n - 2));

// Recursively reads directory and counts letters
const countLettersInDir = async (dir: string): Promise<number> => {
    let totalLetters = 0;

    const entries = await readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
        const fullPath = join(dir, entry.name);

        if (entry.isDirectory()) {
            totalLetters += await countLettersInDir(fullPath); // Recursively process subdirectories
        } else {
            const content = await readFile(fullPath, "utf-8");
            totalLetters += content.replace(/\s/g, "").length; // Count non-space characters
        }
    }

    return totalLetters;
};

// Worker logic
self.onmessage = async (event) => {
    const dir = "../api"; // Change this to your test directory

    fib(20); // Simulate CPU-heavy task

    let totalLetters = 0;
    try {
        totalLetters = await countLettersInDir(dir);
        self.postMessage(totalLetters); // Send to main thread
    } catch (err) {
        console.error(`Worker ${event.data} failed`, err);
    }
};