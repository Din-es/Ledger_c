import { ItemView, WorkspaceLeaf } from "obsidian";
import type { Report, Status } from "./engine";
import type DecisionLedgerPlugin from "./main";
import { statusPill } from "./render";

export const LEDGER_VIEW_TYPE = "decision-ledger-view";

const ORDER: Record<Status, number> = { broken: 0, drifted: 1, fresh: 2 };

export class LedgerView extends ItemView {
	private reports: Report[] = [];
	private error: string | null = null;
	private loading = false;

	constructor(leaf: WorkspaceLeaf, private plugin: DecisionLedgerPlugin) {
		super(leaf);
	}

	getViewType() {
		return LEDGER_VIEW_TYPE;
	}
	getDisplayText() {
		return "Decision ledger";
	}
	getIcon() {
		return "git-commit-horizontal";
	}

	async onOpen() {
		this.render();
		if (this.plugin.settings.resolveOnLoad) await this.refresh();
	}

	async refresh() {
		this.loading = true;
		this.error = null;
		this.render();
		try {
			this.reports = await this.plugin.engine.list();
		} catch (err) {
			this.error = err instanceof Error ? err.message : String(err);
			this.reports = [];
		} finally {
			this.loading = false;
			this.render();
		}
	}

	private render() {
		const root = this.contentEl;
		root.empty();
		root.addClass("ledger-view");

		const toolbar = root.createDiv({ cls: "ledger-toolbar" });
		const refresh = toolbar.createEl("button", {
			text: this.loading ? "Resolving…" : "Resolve all",
		});
		refresh.disabled = this.loading;
		refresh.addEventListener("click", () => void this.refresh());

		if (this.error) {
			root.createDiv({ cls: "ledger-notice ledger-notice-broken", text: this.error });
			return;
		}
		if (this.loading && this.reports.length === 0) return;
		if (this.reports.length === 0) {
			root.createDiv({
				cls: "ledger-empty",
				text: "No decisions recorded yet. Bind one with `ledger bind`.",
			});
			return;
		}

		const counts = { broken: 0, drifted: 0, fresh: 0 } as Record<Status, number>;
		for (const r of this.reports) counts[r.status]++;
		root.createDiv({
			cls: "ledger-summary",
			text: `${this.reports.length} decisions · ${counts.broken} broken · ${counts.drifted} drifted`,
		});

		const sorted = [...this.reports].sort(
			(a, b) => ORDER[a.status] - ORDER[b.status] || a.id.localeCompare(b.id),
		);
		const list = root.createDiv({ cls: "ledger-list" });
		for (const report of sorted) {
			const row = list.createDiv({ cls: "ledger-row" });
			const main = row.createDiv({ cls: "ledger-row-main" });
			main.createDiv({ cls: "ledger-title", text: report.title || report.id });
			main.createDiv({
				cls: "ledger-location",
				text:
					report.status === "broken"
						? report.file
						: `${report.file}:${report.range[0]}-${report.range[1]}`,
			});
			statusPill(row, report.status, report.confidence);
			row.addEventListener("click", () => void this.plugin.openNote(report));
		}
	}
}
