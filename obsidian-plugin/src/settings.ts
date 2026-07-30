import { App, PluginSettingTab, Setting } from "obsidian";
import type DecisionLedgerPlugin from "./main";

export interface LedgerSettings {
	/** Path to the compiled `ledger` binary. */
	binaryPath: string;
	/** Absolute path to the repo whose .ledger/ records we read. */
	repoPath: string;
	/** Re-resolve everything when the vault loads. */
	resolveOnLoad: boolean;
}

export const DEFAULT_SETTINGS: LedgerSettings = {
	binaryPath: "ledger",
	repoPath: "",
	resolveOnLoad: true,
};

export class LedgerSettingTab extends PluginSettingTab {
	constructor(app: App, private plugin: DecisionLedgerPlugin) {
		super(app, plugin);
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();

		new Setting(containerEl)
			.setName("Ledger binary")
			.setDesc("Path to the compiled ledger executable.")
			.addText((text) =>
				text
					.setPlaceholder("ledger")
					.setValue(this.plugin.settings.binaryPath)
					.onChange(async (value) => {
						this.plugin.settings.binaryPath = value.trim();
						await this.plugin.saveSettings();
					}),
			);

		new Setting(containerEl)
			.setName("Repository path")
			.setDesc("Absolute path to the git repo holding .ledger/ records.")
			.addText((text) =>
				text
					.setPlaceholder("C:\\path\\to\\repo")
					.setValue(this.plugin.settings.repoPath)
					.onChange(async (value) => {
						this.plugin.settings.repoPath = value.trim();
						await this.plugin.saveSettings();
					}),
			);

		new Setting(containerEl)
			.setName("Resolve on load")
			.setDesc("Refresh decision statuses when the vault opens.")
			.addToggle((toggle) =>
				toggle.setValue(this.plugin.settings.resolveOnLoad).onChange(async (value) => {
					this.plugin.settings.resolveOnLoad = value;
					await this.plugin.saveSettings();
				}),
			);
	}
}
