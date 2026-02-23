declare global {
  interface Window {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    Go: any;
    renderMarkdown: (source: string) => string;
    checkConflict: (localEtag: string, remoteEtag: string) => boolean;
    createOfflineChange: (
      noteID: string,
      content: string,
    ) => { noteId: string; content: string; timestamp: number };
  }
}

let wasmPromise: Promise<void> | null = null;

export const initWasm = async () => {
  if (typeof window === "undefined") return;

  if (wasmPromise) return wasmPromise;

  wasmPromise = new Promise<void>(async (resolve, reject) => {
    if (!window.Go) {
      console.error("Go runtime not found. Ensure wasm_exec.js is loaded.");
      reject(new Error("Go runtime not found"));
      return;
    }

    const go = new window.Go();

    try {
      const result = await WebAssembly.instantiateStreaming(
        fetch("/core.wasm"),
        go.importObject,
      );
      go.run(result.instance);
      console.log("Wasm initialized");
      resolve();
    } catch (err) {
      console.error("Failed to load Wasm", err);
      reject(err);
    }
  });

  return wasmPromise;
};
