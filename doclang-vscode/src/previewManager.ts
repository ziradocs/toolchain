// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import * as path from 'path';
import { DocLangPreviewPanel } from './previewPanel';

export class DocLangPreviewManager implements vscode.Disposable {
    private readonly _previews: Map<string, DocLangPreviewPanel> = new Map();
    private readonly _disposables: vscode.Disposable[] = [];

    constructor(private readonly context: vscode.ExtensionContext) {}

    public showPreview(uri: vscode.Uri, viewColumn: vscode.ViewColumn): void {
        const key = uri.toString();
        
        // Si ya existe un preview para este archivo, revelarlo
        const existingPreview = this._previews.get(key);
        if (existingPreview) {
            existingPreview.reveal(viewColumn);
            return;
        }

        // Crear nuevo preview
        const preview = new DocLangPreviewPanel(
            uri,
            viewColumn,
            this.context.extensionUri
        );

        this._previews.set(key, preview);

        // Limpiar cuando se cierra el panel
        preview.onDidDispose(() => {
            this._previews.delete(key);
        });
    }

    public updatePreview(uri: vscode.Uri): void {
        const key = uri.toString();
        const preview = this._previews.get(key);
        
        if (preview) {
            preview.update();
        }
    }

    public refreshActivePreview(): void {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            return;
        }

        this.updatePreview(editor.document.uri);
    }

    public dispose(): void {
        // Cerrar todos los previews
        this._previews.forEach(preview => preview.dispose());
        this._previews.clear();

        // Limpiar disposables
        this._disposables.forEach(d => d.dispose());
        this._disposables.length = 0;
    }
}
