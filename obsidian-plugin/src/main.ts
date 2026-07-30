import { Notice, Plugin, TFile, WorkspaceLeaf, parseYaml } from "obsidian";
import { Engine, type Report } from "./engine";
import { renderError, renderReport } from "./render";
import { DEFAULT_SETTINGS, LedgerSettingTab, type LedgerSettings } from "./settings";
import { LEDGER_VIEW_TYPE, LedgerView } from "./view";
import { join } from "path";
import { spawn } from "child_process";

export default class DecisionLedgerPlugin extends Plugin {
	settings: LedgerSettings = DEFAULT_SETTINGS;
	engine!: Engine;

	async onload() {
		await this.loadSettings();
		this.engine = new Engine(this.settings.binaryPath, this.settings.repoPath);

		this.registerView(LEDGER_VIEW_TYPE, (leaf: WorkspaceLeaf) => new LedgerView(leaf, this));
		this.addSettingTab(new LedgerSettingTab(this.app, this));

		// The hero feature: a ```ledger block renders the live code a decision
		// governs, resolved through the engine at read time.
		this.registerMarkdownCodeBlockProcessor("ledger", async (source, el, ctx) => {
			const id = this.parseBlockId(source) ?? (await this.idFromFrontmatter(ctx.sourcePath));
			if (!id) {
				renderError(el, "No decision id. Add `id: your-decision` to this block.");
				return;
			}
			el.createDiv({ cls: "ledger-loading", text: "Resolving…" });
			try {
				const report = await this.engine.resolve(id);
				el.empty();
				if (!report) {
					renderError(el, `No decision named "${id}" in .ledger/.`);
					return;
				}
				renderReport(el, report, (r) => this.openSource(r));
			} catch (err) {
				el.empty();
				renderError(el, err instanceof Error ? err.message : String(err));
			}
		});

		this.addCommand({
			id: "open-ledger-view",
			name: "Open decision ledger",
			callback: () => void this.activateView(),
		});

		this.addCommand({
			id: "resolve-all-decisions",
			name: "Resolve all decisions",
			callback: async () => {
				const view = this.getView();
				if (view) {
					await view.refresh();
					new Notice("Decisions resolved.");
				} else {
					await this.activateView();
				}
			},
		});

		this.addCommand({
			id: "show-stale-decisions",
			name: "Show stale decisions",
			callback: async () => {
				try {
					const reports = await this.engine.list();
					const stale = reports.filter((r) => r.status !== "fresh");
					new Notice(
						stale.length === 0
							? "All decisions track their code."
							: `${stale.length} decision(s) need attention: ${stale.map((r) => r.id).join(", ")}`,
					);
					await this.activateView();
				} catch (err) {
					new Notice(err instanceof Error ? err.message : String(err));
				}
			},
		});
	}

	onunload() {
		// Obsidian detaches views registered via registerView automatically.
	}

	/** Reads `id: foo` out of a ```ledger block body. */
	private parseBlockId(source: string): string | null {
		try {
			const parsed = parseYaml(source);
			if (parsed && typeof parsed.id === "string") return parsed.id.trim();
		} catch {
			// Fall through to the bare-string form below.
		}
		const bare = source.trim();
		return bare && !bare.includes("\n") && !bare.includes(":") ? bare : null;
	}

	/** Falls back to `ledger-id:` in the note's frontmatter. */
	private async idFromFrontmatter(sourcePath: string): Promise<string | null> {
		const file = this.app.vault.getAbstractFileByPath(sourcePath);
		if (!(file instanceof TFile)) return null;
		const cache = this.app.metadataCache.getFileCache(file);
		const id = cache?.frontmatter?.["ledger-id"];
		return typeof id === "string" ? id : null;
	}

	/** Opens the governed code in the system's default handler. */
	private openSource(report: Report) {
		const target = join(this.settings.repoPath, report.file);
		const opener =
			process.platform === "win32" ? "explorer" : process.platform === "darwin" ? "open" : "xdg-open";
		try {
			spawn(opener, [target], { detached: true, stdio: "ignore" }).unref();
		} catch {
			new Notice(`Could not open ${target}`);
		}
	}

	/** Opens the decision's note in the vault, if it maps to a vault file. */
	async openNote(report: Report) {
		if (!report.note) return;
		const file = this.app.vault.getAbstractFileByPath(report.note);
		if (file instanceof TFile) {
			await this.app.workspace.getLeaf(false).openFile(file);
		} else {
			new Notice(`Note not in this vault: ${report.note}`);
		}
	}

	private getView(): LedgerView | null {
		const leaf = this.app.workspace.getLeavesOfType(LEDGER_VIEW_TYPE)[0];
		return leaf?.view instanceof LedgerView ? leaf.view : null;
	}

	async activateView() {
		const existing = this.app.workspace.getLeavesOfType(LEDGER_VIEW_TYPE);
		if (existing.length > 0) {
			await this.app.workspace.revealLeaf(existing[0]);
			return;
		}
		const leaf = this.app.workspace.getRightLeaf(false);
		if (!leaf) return;
		await leaf.setViewState({ type: LEDGER_VIEW_TYPE, active: true });
		await this.app.workspace.revealLeaf(leaf);
	}

	async loadSettings() {
		this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
	}

	async saveSettings() {
		await this.saveData(this.settings);
		this.engine.configure(this.settings.binaryPath, this.settings.repoPath);
	}
}
