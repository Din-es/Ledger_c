import type { Report, Status } from "./engine";

const STATUS_LABEL: Record<Status, string> = {
	fresh: "tracked",
	drifted: "drifted",
	broken: "code gone",
};

/** Status pill shared by the codeblock and the sidebar. */
export function statusPill(parent: HTMLElement, status: Status, confidence?: number): HTMLElement {
	const pill = parent.createSpan({ cls: `ledger-pill ledger-pill-${status}` });
	pill.createSpan({ cls: "ledger-dot" });
	let text = STATUS_LABEL[status];
	if (status === "drifted" && typeof confidence === "number") {
		text += ` ${Math.round(confidence * 100)}%`;
	}
	pill.createSpan({ text });
	return pill;
}

/**
 * Render one resolved decision: header with status, the live code at its
 * current location, and an action when it needs attention.
 */
export function renderReport(
	el: HTMLElement,
	report: Report,
	onOpenSource: (report: Report) => void,
): void {
	const card = el.createDiv({ cls: `ledger-card ledger-card-${report.status}` });

	const header = card.createDiv({ cls: "ledger-header" });
	const heading = header.createDiv({ cls: "ledger-heading" });
	heading.createSpan({ cls: "ledger-title", text: report.title || report.id });

	const location =
		report.status === "broken"
			? report.file
			: `${report.file}:${report.range[0]}-${report.range[1]}`;
	heading.createSpan({ cls: "ledger-location", text: location });

	statusPill(header, report.status, report.confidence);

	if (report.renamed) {
		card.createDiv({
			cls: "ledger-notice",
			text: `Followed a rename — this code now lives at ${report.file}.`,
		});
	}

	if (report.status === "broken") {
		card.createDiv({
			cls: "ledger-notice ledger-notice-broken",
			text: "The code this decision governed is gone. Is the decision still valid?",
		});
		return;
	}

	if (report.status === "drifted") {
		card.createDiv({
			cls: "ledger-notice",
			text: "The governed code changed since this decision was bound. Re-read it, then re-bind if the decision still holds.",
		});
	}

	if (report.code?.length) {
		const pre = card.createEl("pre", { cls: "ledger-code" });
		const code = pre.createEl("code");
		report.code.forEach((line, i) => {
			const row = code.createDiv({ cls: "ledger-line" });
			row.createSpan({ cls: "ledger-lineno", text: String(report.range[0] + i) });
			row.createSpan({ cls: "ledger-linetext", text: line || " " });
		});
	}

	const actions = card.createDiv({ cls: "ledger-actions" });
	const open = actions.createEl("button", { text: "Open source" });
	open.addEventListener("click", () => onOpenSource(report));
}

export function renderError(el: HTMLElement, message: string): void {
	const card = el.createDiv({ cls: "ledger-card ledger-card-broken" });
	card.createDiv({ cls: "ledger-notice ledger-notice-broken", text: message });
}
