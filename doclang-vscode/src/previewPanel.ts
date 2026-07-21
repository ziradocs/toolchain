// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import * as path from 'path';
import { DocLangBuilder } from './doclangBuilder';

export class DocLangPreviewPanel implements vscode.Disposable {
    private readonly _panel: vscode.WebviewPanel;
    private readonly _disposables: vscode.Disposable[] = [];
    private readonly _builder: DocLangBuilder;
    private _isDisposed = false;

    constructor(
        private readonly _uri: vscode.Uri,
        viewColumn: vscode.ViewColumn,
        private readonly _extensionUri: vscode.Uri
    ) {
        this._builder = new DocLangBuilder();

        // Crear webview panel
        this._panel = vscode.window.createWebviewPanel(
            'doclangPreview',
            `Preview: ${path.basename(_uri.fsPath)}`,
            viewColumn,
            {
                enableScripts: true,
                retainContextWhenHidden: true,
                localResourceRoots: [
                    vscode.Uri.joinPath(_extensionUri, 'media'),
                    vscode.Uri.file(path.dirname(_uri.fsPath))
                ]
            }
        );

        // Icono del panel
        this._panel.iconPath = {
            light: vscode.Uri.joinPath(_extensionUri, 'media', 'preview-light.svg'),
            dark: vscode.Uri.joinPath(_extensionUri, 'media', 'preview-dark.svg')
        };

        // Cargar contenido inicial
        this.update();

        // Limpiar cuando se cierra
        this._panel.onDidDispose(() => this.dispose(), null, this._disposables);

        // Manejar mensajes del webview
        this._panel.webview.onDidReceiveMessage(
            message => {
                switch (message.command) {
                    case 'refresh':
                        this.update();
                        return;
                }
            },
            null,
            this._disposables
        );
    }

    public async update(): Promise<void> {
        if (this._isDisposed) {
            return;
        }

        try {
            // Generar HTML usando DocLang CLI
            const html = await this._builder.build(this._uri);
            
            // Actualizar webview con el HTML generado
            this._panel.webview.html = this.getWebviewContent(html);
        } catch (error) {
            const errorMessage = error instanceof Error ? error.message : String(error);
            this._panel.webview.html = this.getErrorContent(errorMessage);
        }
    }

    public reveal(viewColumn: vscode.ViewColumn): void {
        this._panel.reveal(viewColumn);
    }

    public onDidDispose(callback: () => void): vscode.Disposable {
        return this._panel.onDidDispose(callback);
    }

    public dispose(): void {
        if (this._isDisposed) {
            return;
        }

        this._isDisposed = true;
        this._panel.dispose();

        while (this._disposables.length) {
            const disposable = this._disposables.pop();
            if (disposable) {
                disposable.dispose();
            }
        }
    }

    private getWebviewContent(doclangHtml: string): string {
        // Extraer solo el contenido del body (sin el wrapper completo)
        const bodyMatch = doclangHtml.match(/<body[^>]*>([\s\S]*)<\/body>/i);
        const content = bodyMatch ? bodyMatch[1] : doclangHtml;

        // Extraer estilos del head
        const styleMatch = doclangHtml.match(/<style[^>]*>([\s\S]*?)<\/style>/gi);
        const styles = styleMatch ? styleMatch.join('\n') : '';

        // Extraer scripts del head/body
        const scriptMatch = doclangHtml.match(/<script[^>]*src="([^"]+)"[^>]*><\/script>/gi);
        const scripts = scriptMatch ? scriptMatch.join('\n') : '';

        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; 
        style-src 'unsafe-inline' https:; 
        script-src 'unsafe-inline' 'unsafe-eval' https:; 
        img-src vscode-resource: https: data:;
        font-src https:;
        connect-src https:;">
    <title>DocLang Preview</title>
    ${styles}
    <style>
        body {
            padding: 20px;
            background: white;
        }
        .refresh-button {
            position: fixed;
            top: 10px;
            right: 10px;
            padding: 8px 16px;
            background: #007acc;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            z-index: 1000;
        }
        .refresh-button:hover {
            background: #005a9e;
        }
    </style>
</head>
<body>
    <button class="refresh-button" onclick="refresh()">↻ Refresh</button>
    ${content}
    
    ${scripts}
    
    <script>
        const vscode = acquireVsCodeApi();
        
        function refresh() {
            vscode.postMessage({ command: 'refresh' });
        }
        
        // Auto-scroll to preserve position
        window.addEventListener('load', () => {
            const state = vscode.getState();
            if (state && state.scrollTop) {
                window.scrollTo(0, state.scrollTop);
            }
        });
        
        // Save scroll position
        window.addEventListener('scroll', () => {
            vscode.setState({ scrollTop: window.scrollY });
        });
    </script>
</body>
</html>`;
    }

    private getErrorContent(errorMessage: string): string {
        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Preview Error</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            padding: 40px;
            background: #f5f5f5;
        }
        .error-container {
            background: white;
            border-left: 4px solid #e74c3c;
            padding: 20px;
            border-radius: 4px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            color: #e74c3c;
            margin-top: 0;
        }
        pre {
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 15px;
            border-radius: 4px;
            overflow-x: auto;
        }
        .retry-button {
            margin-top: 20px;
            padding: 10px 20px;
            background: #3498db;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .retry-button:hover {
            background: #2980b9;
        }
    </style>
</head>
<body>
    <div class="error-container">
        <h1>⚠️ Preview Error</h1>
        <p>Failed to generate DocLang preview:</p>
        <pre>${this.escapeHtml(errorMessage)}</pre>
        <button class="retry-button" onclick="retry()">🔄 Retry</button>
    </div>
    
    <script>
        const vscode = acquireVsCodeApi();
        
        function retry() {
            vscode.postMessage({ command: 'refresh' });
        }
    </script>
</body>
</html>`;
    }

    private escapeHtml(text: string): string {
        return text
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }
}
