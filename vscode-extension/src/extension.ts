import * as vscode from "vscode";
import { execFile } from "child_process";
import * as path from "path";

type Status = "fresh" | "drifted" | "broken";

interface Report {
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

const LABEL: Record<Status, string> = {
	fresh: "tracked",
	drifted: "drifted",
	broken: "code gone",
};

let reports: Report[] = [];
let decorations: Record<Status, vscode.TextEditorDecorationType>;
const onDidChange = new vscode.EventEmitter<void>();

export function activate(context: vscode.ExtensionContext) {
	decorations = {
		fresh: makeDecoration(context, "fresh.svg", "#1baf7a"),
		drifted: makeDecoration(context, "drifted.svg", "#eda100"),
		broken: makeDecoration(context, "broken.svg", "#e34948"),
	};
	context.subscriptions.push(...Object.values(decorations));

	context.subscriptions.push(
		vscode.commands.registerCommand("decisionLedger.refresh", () => refresh()),
		vscode.commands.registerCommand("decisionLedger.why", showWhy),
		vscode.commands.registerCommand("decisionLedger.bind", bindSelection),
		vscode.commands.registerCommand("decisionLedger.openNote", openNote),
		vscode.languages.registerHoverProvider({ scheme: "file" }, { provideHover }),
		vscode.languages.registerCodeLensProvider({ scheme: "file" }, new LedgerCodeLensProvider()),
		vscode.window.onDidChangeActiveTextEditor(applyDecorations),
		vscode.workspace.onDidSaveTextDocument(() => refresh()),
	);

	void refresh();
}

export function deactivate() {}

function makeDecoration(
	context: vscode.ExtensionContext,
	icon: string,
	color: string,
): vscode.TextEditorDecorationType {
	return vscode.window.createTextEditorDecorationType({
		gutterIconPath: vscode.Uri.file(path.join(context.extensionPath, "media", icon)),
		gutterIconSize: "60%",
		overviewRulerColor: color,
		overviewRulerLane: vscode.OverviewRulerLane.Right,
		isWholeLine: true,
	});
}

/** Repo root — the ledger CLI is run from here so it finds .ledger/. */
function repoRoot(): string | undefined {
	return vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
}

/** Runs the CLI and returns raw stdout. */
function runLedgerRaw(args: string[]): Promise<string> {
	const cwd = repoRoot();
	return new Promise((resolve, reject) => {
		if (!cwd) {
			reject(new Error("Open a folder first."));
			return;
		}
		const binary = vscode.workspace
			.getConfiguration("decisionLedger")
			.get<string>("binaryPath", "ledger");
		execFile(binary, args, { cwd, windowsHide: true, maxBuffer: 8 * 1024 * 1024 }, (err, stdout, stderr) => {
			// `verify` exits non-zero by design, so only treat an error with no
			// output as a real failure.
			if (err && !stdout.trim()) {
				reject(new Error(stderr.trim() || err.message));
				return;
			}
			resolve(stdout);
		});
	});
}

async function runLedger(args: string[]): Promise<Report[]> {
	if (!repoRoot()) return [];
	const stdout = await runLedgerRaw(args);
	try {
		return JSON.parse(stdout || "[]");
	} catch {
		throw new Error("Could not parse ledger output.");
	}
}

async function refresh() {
	try {
		reports = await runLedger(["list", "--json"]);
	} catch (err) {
		reports = [];
		// Stay quiet on startup when the binary or repo isn't configured yet;
		// the explicit refresh command surfaces the error.
		console.warn("decision-ledger:", err instanceof Error ? err.message : err);
	}
	onDidChange.fire();
	vscode.window.visibleTextEditors.forEach(applyDecorations);
}

/** Reports whose resolved span sits in the given document. */
function reportsFor(doc: vscode.TextDocument): Report[] {
	const root = repoRoot();
	if (!root) return [];
	const rel = path.relative(root, doc.uri.fsPath).split(path.sep).join("/");
	return reports.filter((r) => r.status !== "broken" && r.file.split(/[\\/]/).join("/") === rel);
}

function toRange(r: Report): vscode.Range {
	// Engine ranges are 1-based inclusive; VS Code is 0-based.
	return new vscode.Range(r.range[0] - 1, 0, r.range[1] - 1, Number.MAX_SAFE_INTEGER);
}

function applyDecorations(editor: vscode.TextEditor | undefined) {
	if (!editor) return;
	const mine = reportsFor(editor.document);
	for (const status of ["fresh", "drifted", "broken"] as Status[]) {
		editor.setDecorations(
			decorations[status],
			mine.filter((r) => r.status === status).map((r) => ({
				range: toRange(r),
				hoverMessage: hoverFor(r),
			})),
		);
	}
}

function hoverFor(r: Report): vscode.MarkdownString {
	const md = new vscode.MarkdownString(undefined, true);
	md.isTrusted = true;
	const pct = Math.round(r.confidence * 100);
	md.appendMarkdown(`**${r.title || r.id}**\n\n`);
	md.appendMarkdown(
		`$(git-commit) ${LABEL[r.status]}${r.status === "drifted" ? ` · ${pct}% match` : ""} · bound at \`${r.boundAt.slice(0, 7)}\`\n\n`,
	);
	if (r.status === "drifted") {
		md.appendMarkdown(`_This code changed since the decision was written — re-read it._\n\n`);
	}
	if (r.renamed) {
		md.appendMarkdown(`_Followed a rename to \`${r.file}\`._\n\n`);
	}
	if (r.note) {
		const args = encodeURIComponent(JSON.stringify([r.note]));
		md.appendMarkdown(`[Open decision note](command:decisionLedger.openNote?${args})`);
	}
	return md;
}

async function provideHover(
	doc: vscode.TextDocument,
	pos: vscode.Position,
): Promise<vscode.Hover | undefined> {
	const hit = reportsFor(doc).find((r) => pos.line >= r.range[0] - 1 && pos.line <= r.range[1] - 1);
	return hit ? new vscode.Hover(hoverFor(hit), toRange(hit)) : undefined;
}

class LedgerCodeLensProvider implements vscode.CodeLensProvider {
	onDidChangeCodeLenses = onDidChange.event;

	provideCodeLenses(doc: vscode.TextDocument): vscode.CodeLens[] {
		if (!vscode.workspace.getConfiguration("decisionLedger").get<boolean>("showCodeLens", true)) {
			return [];
		}
		return reportsFor(doc).map((r) => {
			const line = new vscode.Range(r.range[0] - 1, 0, r.range[0] - 1, 0);
			const mark = r.status === "fresh" ? "why" : `why (${LABEL[r.status]})`;
			return new vscode.CodeLens(line, {
				title: `${mark}: ${r.title || r.id}`,
				command: r.note ? "decisionLedger.openNote" : "",
				arguments: r.note ? [r.note] : [],
			});
		});
	}
}

/** The command palette entry: explain the decision at the cursor. */
async function showWhy() {
	const editor = vscode.window.activeTextEditor;
	if (!editor) return;
	const root = repoRoot();
	if (!root) {
		vscode.window.showWarningMessage("Decision Ledger: open a folder first.");
		return;
	}
	const rel = path.relative(root, editor.document.uri.fsPath).split(path.sep).join("/");
	const line = editor.selection.active.line + 1;

	try {
		const hits = await runLedger(["why", `${rel}:${line}`, "--json"]);
		if (hits.length === 0) {
			vscode.window.showInformationMessage(`No decision governs ${rel}:${line}.`);
			return;
		}
		const pick = await vscode.window.showQuickPick(
			hits.map((r) => ({
				label: r.title || r.id,
				description: `${LABEL[r.status]} · ${r.file}:${r.range[0]}-${r.range[1]}`,
				detail: r.note,
				report: r,
			})),
			{ placeHolder: "Decisions governing this line" },
		);
		if (pick?.report.note) await openNote(pick.report.note);
	} catch (err) {
		vscode.window.showErrorMessage(
			`Decision Ledger: ${err instanceof Error ? err.message : String(err)}`,
		);
	}
}

/**
 * Bind the current selection to a decision. This is the capture half of the
 * loop — if writing a decision down costs more than a right-click, nobody
 * does it and the ledger stays empty.
 */
async function bindSelection() {
	const editor = vscode.window.activeTextEditor;
	const root = repoRoot();
	if (!editor || !root) return;
	if (editor.selection.isEmpty) {
		vscode.window.showWarningMessage("Decision Ledger: select the code the decision governs first.");
		return;
	}

	const rel = path.relative(root, editor.document.uri.fsPath).split(path.sep).join("/");
	const start = editor.selection.start.line + 1;
	const end = editor.selection.end.line + 1;

	// Offer to extend an existing decision, since one rationale often governs
	// code in several places.
	const existing = [...new Map(reports.map((r) => [r.id, r])).values()];
	const NEW = "$(add) New decision…";
	const choice = await vscode.window.showQuickPick(
		[NEW, ...existing.map((r) => `$(git-commit) ${r.id} — ${r.title ?? ""}`)],
		{ placeHolder: `Bind ${rel}:${start}-${end} to…` },
	);
	if (!choice) return;

	let id: string | undefined;
	let title: string | undefined;
	const isNew = choice === NEW;

	if (isNew) {
		id = await vscode.window.showInputBox({
			prompt: "Decision id (kebab-case)",
			placeHolder: "jitter-backoff",
			validateInput: (v) =>
				/^[a-z0-9][a-z0-9-]*$/.test(v.trim()) ? null : "Use lowercase letters, digits and dashes.",
		});
		if (!id) return;
		title = await vscode.window.showInputBox({
			prompt: "One-line summary of the decision",
			placeHolder: "Jittered backoff avoids thundering herd",
		});
		if (title === undefined) return;
	} else {
		id = choice.replace(/^\$\(git-commit\) /, "").split(" — ")[0];
	}

	const args = ["bind", `${rel}:${start}-${end}`, "--note", id.trim()];
	if (title) args.push("--title", title);
	if (!isNew) args.push("--add");

	try {
		const out = await runLedgerRaw(args);
		await refresh();
		const notePath = `docs/decisions/${id.trim()}.md`;
		const action = await vscode.window.showInformationMessage(
			out.trim() || `Bound ${id}`,
			"Open note",
		);
		if (action === "Open note") await openNote(notePath, true);
	} catch (err) {
		vscode.window.showErrorMessage(
			`Decision Ledger: ${err instanceof Error ? err.message : String(err)}`,
		);
	}
}

async function openNote(note: string, createIfMissing = false) {
	const root = repoRoot();
	if (!root) return;
	const uri = vscode.Uri.file(path.join(root, note));
	try {
		await vscode.window.showTextDocument(await vscode.workspace.openTextDocument(uri));
	} catch {
		if (!createIfMissing) {
			vscode.window.showWarningMessage(`Decision Ledger: could not open ${note}`);
			return;
		}
		// A freshly bound decision has no note yet — start one from a template
		// so there is somewhere to write the rationale immediately.
		const id = path.basename(note, ".md");
		const stub = [
			`---`,
			`ledger-id: ${id}`,
			`---`,
			``,
			`# ${id}`,
			``,
			`## Context`,
			``,
			`What made this decision necessary?`,
			``,
			`## Decision`,
			``,
			`What did you choose?`,
			``,
			`## The code this governs`,
			``,
			'```ledger',
			`id: ${id}`,
			'```',
			``,
			`## Consequences`,
			``,
			`What does this cost, and what does it buy?`,
			``,
		].join("\n");
		await vscode.workspace.fs.createDirectory(vscode.Uri.file(path.dirname(uri.fsPath)));
		await vscode.workspace.fs.writeFile(uri, Buffer.from(stub, "utf8"));
		await vscode.window.showTextDocument(await vscode.workspace.openTextDocument(uri));
	}
}
