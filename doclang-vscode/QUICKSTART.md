# DocLang VS Code Extension - Quick Start Guide

## 🚀 Inicio Rápido

### 1. Instalación

La extensión está en el workspace, para usarla:

```bash
# Desde la raíz del proyecto
cd doclang-vscode

# Presiona F5 en VS Code para lanzar Extension Development Host
# O usa el menú: Run > Start Debugging
```

Esto abrirá una nueva ventana de VS Code con la extensión cargada.

### 2. Configuración del CLI

La extensión busca automáticamente el CLI de DocLang en:
1. El path configurado en settings
2. La carpeta `doclang/` del workspace
3. El PATH del sistema

**Configuración recomendada** (en la ventana de desarrollo):

```json
{
  "doclang.executablePath": "/Users/mmonterroca/cli/doclang/doclang"
}
```

### 3. Uso Básico

1. **Abrir un archivo Markdown** (cualquier `.md`)

2. **Abrir Preview**:
   - Presiona `Cmd+Shift+P` (Mac) o `Ctrl+Shift+P` (Windows/Linux)
   - Escribe: `DocLang: Open Preview to the Side`
   - O haz clic en el icono de preview en la barra superior del editor

3. **Editar y Ver en Tiempo Real**:
   - Edita tu archivo `.md`
   - Guarda (`Cmd+S` / `Ctrl+S`)
   - El preview se actualiza automáticamente

## 📝 Ejemplo de Prueba

Crea un archivo `test-preview.md`:

```markdown
---
mode: flex
title: "Test de DocLang Preview"
---

# Bienvenido a DocLang Preview

Este es un test de la extensión de VS Code.

## Características

### Listas
- ✅ Markdown básico
- ✅ **Negrita** y *cursiva*
- ✅ `Código inline`

### Bloques de Código

\`\`\`javascript
function hello() {
  console.log("Hello from DocLang!");
}
\`\`\`

### Tablas

| Feature | Status |
|---------|--------|
| Preview | ✅ Working |
| Auto-refresh | ✅ Working |
| TOC | ✅ Working |

### Diagramas Mermaid

\`\`\`mermaid
graph LR
    A[Edit MD] --> B[Save]
    B --> C[Auto Refresh]
    C --> D[See Preview]
\`\`\`

## Elementos Avanzados

### Bloques Especiales

:::info
Esto es un bloque de información.
:::

:::warning
Esto es una advertencia.
:::

:::success
¡Éxito! La extensión funciona correctamente.
:::

### Imágenes

![Logo](https://via.placeholder.com/150)

### Enlaces

Visita la [documentación de DocLang](../docs/doclang/)
```

## 🎯 Comandos Disponibles

| Comando | Descripción | Atajo |
|---------|-------------|-------|
| `DocLang: Open Preview` | Abrir preview en columna activa | - |
| `DocLang: Open Preview to the Side` | Abrir preview al lado | - |
| `DocLang: Refresh Preview` | Refrescar manualmente | - |

## ⚙️ Configuración

### Todas las opciones disponibles:

```json
{
  // Ruta al ejecutable de DocLang CLI
  "doclang.executablePath": "doclang",
  
  // Auto-refresh al guardar
  "doclang.autoRefresh": true,
  
  // Habilitar tabla de contenidos
  "doclang.tocEnabled": true,
  
  // Dónde abrir el preview: "beside" o "active"
  "doclang.previewColumn": "beside"
}
```

### Ejemplo de configuración personalizada:

```json
{
  "doclang.executablePath": "/Users/you/cli/doclang/doclang",
  "doclang.autoRefresh": true,
  "doclang.tocEnabled": true,
  "doclang.previewColumn": "beside"
}
```

## 🐛 Troubleshooting

### "DocLang CLI not found"

**Solución 1**: Configurar el path absoluto
```json
{
  "doclang.executablePath": "/ruta/absoluta/al/doclang"
}
```

**Solución 2**: Verificar que el CLI esté compilado
```bash
cd doclang
go build -o doclang cmd/doclang/main.go
```

### Preview no se actualiza

1. Verifica que `autoRefresh` esté habilitado
2. Refresca manualmente: `DocLang: Refresh Preview`
3. Verifica que el archivo se esté guardando (Cmd+S / Ctrl+S)

### Errores en el preview

1. Revisa el error mostrado en el panel de preview
2. Haz clic en el botón "🔄 Retry"
3. Verifica que el archivo `.md` tenga sintaxis válida

## 🎨 Features Soportadas

La extensión soporta **TODOS** los elementos de DocLang:

- ✅ Markdown básico (headings, listas, énfasis, links)
- ✅ Código con syntax highlighting
- ✅ Tablas
- ✅ Imágenes
- ✅ Bloques especiales (info, warning, success, danger)
- ✅ Diagramas Mermaid
- ✅ Charts interactivos
- ✅ Mapas interactivos
- ✅ Code groups con tabs
- ✅ Tabla de contenidos automática
- ✅ Temas profesionales

## 🚀 Desarrollo

### Compilar en modo watch:

```bash
cd doclang-vscode
npm run watch
```

### Ejecutar tests:

```bash
npm test
```

### Empaquetar extensión:

```bash
npm run package
```

## 📚 Recursos

- [README de la extensión](./README.md)
- [Documentación de DocLang](../docs/doclang/)
- [VS Code Extension API](https://code.visualstudio.com/api)

## 🎥 Demo Workflow

```
1. Abrir VS Code en /Users/mmonterroca/cli
2. Presionar F5 para lanzar Extension Development Host
3. En la nueva ventana, abrir docs/doclang/DOCLANG_OVERVIEW.md
4. Cmd+Shift+P > "DocLang: Open Preview to the Side"
5. Ver el preview renderizado con TOC, Mermaid, etc.
6. Editar el archivo
7. Guardar (Cmd+S)
8. Ver el preview actualizarse automáticamente
```

¡Disfruta de DocLang Preview! 🎉
