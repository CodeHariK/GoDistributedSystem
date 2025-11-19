// import { Elysia } from "elysia";

// const app = new Elysia().get("/", () => "Hello Elysia").listen(8080);

// console.log(
//   `🦊 Elysia is running at ${app.server?.hostname}:${app.server?.port}`
// );


import { Elysia } from "elysia";
import { Worker } from "worker_threads"; // Use worker threads for true parallelism

const app = new Elysia();

const runWorker = async (id: number) => {
  return new Promise((resolve, reject) => {
    const worker = new Worker("./src/worker.ts"); // Load separate worker file

    worker.postMessage(id);

    worker.on("message", (message) => {
      console.log(`${message}`);
      resolve(message);
      worker.terminate();
    });

    worker.on("error", reject);
  });
};

app.get("/", async () => {
  const tasks = [];
  for (let i = 0; i < 3; i++) {
    tasks.push(runWorker(i)); // Run workers in parallel
  }

  const results = await Promise.all(tasks);
  const totalLetters = results.reduce((sum: any, count: any) => sum + count, 0);
  return `Bun: All workers finished, total letters: ${totalLetters}\n`;
});

app.listen(8080);
console.log("Server running on http://localhost:8080");
