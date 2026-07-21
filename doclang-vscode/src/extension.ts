// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import { DocLangPreviewPanel } from './previewPanel';
import { DocLangPreviewManager } from './previewManager';

export function activate(context: vscode.ExtensionContext) {
    console.log('DocLang extension is now active!');

    const previewManager = new DocLangPreviewManager(context);

    // Comando: Abrir preview
    const previewCommand = vscode.commands.registerCommand('doclang.preview', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showWarningMessage('No active markdown file');
            return;
        }

        if (editor.document.languageId !== 'markdown') {
            vscode.window.showWarningMessage('This command only works with Markdown files');
            return;
        }

        previewManager.showPreview(editor.document.uri, vscode.ViewColumn.Active);
    });

    // Comando: Abrir preview al lado
    const previewToSideCommand = vscode.commands.registerCommand('doclang.previewToSide', () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showWarningMessage('No active markdown file');
            return;
        }

        if (editor.document.languageId !== 'markdown') {
            vscode.window.showWarningMessage('This command only works with Markdown files');
            return;
        }

        previewManager.showPreview(editor.document.uri, vscode.ViewColumn.Beside);
    });

    // Comando: Refrescar preview
    const refreshCommand = vscode.commands.registerCommand('doclang.refreshPreview', () => {
        previewManager.refreshActivePreview();
    });

    // Watch para cambios en archivos markdown
    const fileWatcher = vscode.workspace.onDidSaveTextDocument((document) => {
        if (document.languageId === 'markdown') {
            const config = vscode.workspace.getConfiguration('doclang');
            const autoRefresh = config.get<boolean>('autoRefresh', true);
            
            if (autoRefresh) {
                previewManager.updatePreview(document.uri);
            }
        }
    });

    // Registrar comandos
    context.subscriptions.push(
        previewCommand,
        previewToSideCommand,
        refreshCommand,
        fileWatcher,
        previewManager
    );
}

export function deactivate() {
    console.log('DocLang extension is now deactivated');
}
