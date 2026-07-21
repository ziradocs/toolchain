# Mermaid Rule Tests

## Descripción
Este directorio contiene tests para la regla `MermaidRule` que convierte bloques de código Mermaid de formato markdown (`````mermaid`) al formato SlideLang (`<<mermaid>>`).

## Estructura de Tests

### Tests Unitarios
- **TestMermaidRule_Apply**: Tests básicos de transformación
- **TestMermaidRule_CleanMermaidContent**: Tests de la función de limpieza
- **TestMermaidRule_Metadata**: Tests de metadatos de la regla

### Tests de Integración  
- **TestMermaidRule_Integration**: Usa archivos reales en `testdata/`

### Benchmarks
- **BenchmarkMermaidRule_Apply**: Medición de performance

## Ejecutar Tests

### Todos los tests de la regla:
```bash
go test -v ./internal/parser/ai/normalizer/rules/enhancement/ -run TestMermaidRule
```

### Solo tests de integración:
```bash
go test -v ./internal/parser/ai/normalizer/rules/enhancement/ -run TestMermaidRule_Integration
```

### Solo benchmarks:
```bash
go test -bench=BenchmarkMermaidRule ./internal/parser/ai/normalizer/rules/enhancement/
```

### Con coverage:
```bash
go test -cover ./internal/parser/ai/normalizer/rules/enhancement/
```

## Archivos Testdata

- `mermaid_input.slidelang`: Archivo de entrada con bloques ```mermaid
- `mermaid_expected.slidelang`: Resultado esperado con bloques <<mermaid>>
- `mermaid_actual.slidelang`: Generado automáticamente si el test falla (para debugging)

## Convenciones

Los archivos en `testdata/` siguen las convenciones estándar de Go:
- No se compilan como parte del package
- Se incluyen en `go test` automáticamente
- Deben usar extensiones apropiadas (.slidelang en este caso)
- Se mantienen bajo control de versiones para reproducibilidad

## Ejemplo de Conversión

**Entrada:**
```markdown
```mermaid
graph TD
    A --> B
```

**Salida:**
```plaintext
<<mermaid>>
  graph TD
  A --> B
```
