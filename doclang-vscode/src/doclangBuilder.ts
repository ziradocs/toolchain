// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';
import { promisify } from 'util';
import { exec } from 'child_process';

const execAsync = promisify(exec);
const mkdirAsync = promisify(fs.mkdir);
const readFileAsync = promisify(fs.readFile);
const unlinkAsync = promisify(fs.unlink);

export class DocLangBuilder {
    private readonly outputDir: string;

    constructor() {
        // Usar directorio temporal para outputs
        this.outputDir = path.join(
            require('os').tmpdir(),
            'doclang-vscode-previews'
        );
        this.ensureOutputDir();
    }

    private async ensureOutputDir(): Promise<void> {
        try {
            await mkdirAsync(this.outputDir, { recursive: true });
        } catch (error) {
            // Directorio ya existe
        }
    }

    public async build(uri: vscode.Uri): Promise<string> {
        const config = vscode.workspace.getConfiguration('doclang');
        const executable = config.get<string>('executablePath', 'doclang');
        const tocEnabled = config.get<boolean>('tocEnabled', true);

        const inputPath = uri.fsPath;
        const outputName = path.basename(inputPath, '.md');
        const outputPath = path.join(this.outputDir, outputName);

        // Construir comando DocLang
        const tocFlag = tocEnabled ? '--toc' : '';
        const command = `"${executable}" build "${inputPath}" --output "${outputPath}" ${tocFlag}`.trim();

        try {
            // Ejecutar DocLang CLI
            await execAsync(command, {
                cwd: path.dirname(inputPath),
                timeout: 30000 // 30 segundos timeout
            });

            // Leer el HTML generado
            const htmlFile = path.join(outputPath, `${outputName}.html`);
            const html = await readFileAsync(htmlFile, 'utf-8');

            // Limpiar archivo temporal
            try {
                await unlinkAsync(htmlFile);
            } catch {
                // Ignorar errores de limpieza
            }

            return html;
        } catch (error) {
            const errorMessage = error instanceof Error ? error.message : String(error);
            
            // Intentar buscar el doclang CLI en el workspace
            if (errorMessage.includes('not found') || errorMessage.includes('command not found')) {
                const workspacePath = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
                if (workspacePath) {
                    const localDoclang = path.join(workspacePath, 'doclang', 'doclang');
                    if (fs.existsSync(localDoclang)) {
                        // Reintentar con la ruta local
                        return this.buildWithExecutable(localDoclang, inputPath, outputPath, tocEnabled);
                    }
                }

                throw new Error(
                    `DocLang CLI not found. Please install DocLang or configure the path in settings.\n\n` +
                    `Current path: ${executable}\n` +
                    `Error: ${errorMessage}`
                );
            }

            throw new Error(`Failed to build DocLang preview:\n${errorMessage}`);
        }
    }

    private async buildWithExecutable(
        executable: string,
        inputPath: string,
        outputPath: string,
        tocEnabled: boolean
    ): Promise<string> {
        const outputName = path.basename(inputPath, '.md');
        const tocFlag = tocEnabled ? '--toc' : '';
        const command = `"${executable}" build "${inputPath}" --output "${outputPath}" ${tocFlag}`.trim();

        const { stdout, stderr } = await execAsync(command, {
            cwd: path.dirname(inputPath),
            timeout: 30000
        });

        if (stderr && !stderr.includes('ℹ️')) {
            console.error('DocLang stderr:', stderr);
        }

        const htmlFile = path.join(outputPath, `${outputName}.html`);
        const html = await readFileAsync(htmlFile, 'utf-8');

        try {
            await unlinkAsync(htmlFile);
        } catch {
            // Ignorar errores de limpieza
        }

        return html;
    }
}
