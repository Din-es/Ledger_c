import { execFile } from "child_process";

export type Status = "fresh" | "drifted" | "broken";

/** One resolved decision, mirroring ledger.Report from the Go engine. */
export interface Report {
	id: string;
	title?: string;
	note?: string;
	file: string;
	range: [number, number];
	status: Status;
	confidence: number;
	renamed?: boolean;
	boundAt: string;
	code?: string[];
}

export class EngineError extends Error {}

/**
 * Thin client over the `ledger` CLI. All git work happens in the Go engine so
 * the plugin stays responsive; we only spawn, parse, and cache.
 */
export class Engine {
	constructor(private binary: string, private repoPath: string) {}

	configure(binary: string, repoPath: string) {
		this.binary = binary;
		this.repoPath = repoPath;
	}

	/** Resolve every decision in the repo against the working tree. */
	list(): Promise<Report[]> {
		return this.run(["list", "--json"]);
	}

	/** Resolve a single decision by id. */
	async resolve(id: string): Promise<Report | null> {
		const reports = await this.run(["resolve", id, "--json"]);
		return reports.length > 0 ? reports[0] : null;
	}

	private run(args: string[]): Promise<Report[]> {
		return new Promise((resolve, reject) => {
			if (!this.binary) {
				reject(new EngineError("No ledger binary configured — set it in plugin settings."));
				return;
			}
			execFile(
				this.binary,
				args,
				{ cwd: this.repoPath, windowsHide: true, maxBuffer: 8 * 1024 * 1024 },
				(err, stdout, stderr) => {
					// `verify` exits non-zero by design; list/resolve only do so on
					// real failure, so surface stderr rather than the exit code.
					if (err && !stdout.trim()) {
						reject(new EngineError(stderr.trim() || err.message));
						return;
					}
					try {
						const parsed = JSON.parse(stdout || "[]");
						resolve(Array.isArray(parsed) ? parsed : [parsed]);
					} catch {
						reject(new EngineError(`Could not parse engine output: ${stdout.slice(0, 200)}`));
					}
				},
			);
		});
	}
}
