# Plan de Implementación: Acción de Aprobaciones IssueOps

## Etapa 1: Fundamentos del Proyecto
**Objetivo**: Establecer la estructura del proyecto, módulo Go y CI básico
**Criterios de Éxito**:
- El módulo Go se inicializa correctamente
- Estructura básica del proyecto establecida
- CI se ejecuta y pasa
- El archivo de metadatos de la acción es válido

**Pruebas**:
- `go build ./...` se ejecuta con éxito
- `go test ./...` pasa (incluso si aún no hay pruebas)
- La validación de sintaxis de GitHub Action pasa

**Estado**: Completo

### Tareas
1. Inicializar el módulo Go (`go mod init github.com/owner/issueops-approvals`)
2. Crear la estructura de directorios según ARCHITECTURE.md
3. Crear `action.yml` con definiciones de entrada/salida
4. Crear `Dockerfile` para la acción basada en Docker
5. Configurar el flujo de trabajo de CI de GitHub Actions
6. Añadir un Makefile básico para el desarrollo local

---

## Etapa 2: Sistema de Configuración
**Objetivo**: Analizar y validar archivos de configuración `.github/approvals.yml`
**Criterios de Éxito**:
- Puede analizar configuraciones YAML válidas en estructuras Go
- Valida campos y estructuras requeridos
- Devuelve mensajes de error claros para configuraciones inválidas
- Soporta configuraciones mínimas y completas

**Pruebas**:
- Analizar configuración mínima (solo políticas + flujos de trabajo)
- Analizar configuración completa con todas las características
- Rechazar configuración con campos requeridos faltantes
- Rechazar configuración con referencias de políticas inválidas
- Manejar formatos `team:name` y nombre de usuario simple

**Estado**: Completo

### Tareas
1. Definir tipos Go para el esquema de configuración (`internal/config/types.go`)
2. Implementar análisis YAML con `gopkg.in/yaml.v3`
3. Añadir lógica de validación para:
   - Campos requeridos (versión, políticas, flujos de trabajo)
   - Referencias de políticas en flujos de trabajo existentes
   - Validación de formato de aprobador
4. Crear esquema JSON para autocompletado en IDE
5. Escribir pruebas exhaustivas de análisis de configuración

---

## Etapa 3: Cliente API de GitHub
**Objetivo**: Envoltorio alrededor de la API de GitHub para issues, comentarios, equipos y etiquetas
**Criterios de Éxito**:
- Puede crear/actualizar/cerrar issues
- Puede leer y analizar comentarios
- Puede listar miembros del equipo (con token de App)
- Puede crear etiquetas y lanzamientos
- Maneja la limitación de tasa de manera eficiente

**Pruebas**:
- Crear issue con etiquetas y asignados
- Leer comentarios de un issue
- Analizar palabras clave de aprobación de comentarios
- Simular verificaciones de membresía de equipo
- Crear etiqueta git anotada

**Estado**: Completo

### Tareas
1. Crear envoltorio del cliente de GitHub (`internal/github/client.go`)
2. Implementar operaciones de issues (`internal/github/issues.go`):
   - CreateIssue, UpdateIssue, CloseIssue
   - AddLabels, AddAssignees
   - ListComments
3. Implementar operaciones de equipo (`internal/github/teams.go`):
   - GetTeamMembers (requiere token de App)
   - IsUserInTeam
4. Implementar operaciones de etiquetas (`internal/github/tags.go`):
   - CreateTag
   - ValidateTagDoesNotExist
5. Manejar autenticación (GITHUB_TOKEN vs token de App)
6. Añadir lógica de reintento para límites de tasa

---

## Etapa 4: Motor Semver
**Objetivo**: Analizar, validar y opcionalmente auto-incrementar versiones semánticas
**Criterios de Éxito**:
- Valida formato semver (con/sin prefijo 'v')
- Soporta versiones pre-lanzamiento (v1.2.3-beta.1)
- Puede incrementar mayor/menor/parche basado en estrategia
- Genera nombres de etiquetas git válidos

**Pruebas**:
- Analizar "1.2.3" y "v1.2.3"
- Analizar pre-lanzamiento "1.2.3-alpha.1"
- Rechazar formatos inválidos
- Incrementar parche: 1.2.3 → 1.2.4
- Incrementar con prefijo: v1.2.3 → v1.2.4

**Estado**: Completo

### Tareas
1. Implementar análisis semver (`internal/semver/parse.go`)
2. Añadir lógica de validación (`internal/semver/validate.go`)
3. Implementar estrategias de incremento (`internal/semver/increment.go`):
   - `input`: Usar versión proporcionada
   - `auto`: Incrementar basado en etiquetas
   - `conventional`: Analizar desde mensajes de commit (futuro)
4. Añadir generación de nombres de etiquetas con prefijo configurable
5. Escribir pruebas exhaustivas de semver

---

## Etapa 5: Motor de Aprobación - Grupo Único
**Objetivo**: Lógica central de aprobación para un solo grupo de aprobación
**Criterios de Éxito**:
- Rastrea aprobaciones por issue
- Cuenta aprobaciones hacia el umbral
- Detecta cuando se alcanza el umbral
- Maneja palabras clave de aprobación/denegación
- Previene auto-aprobación (si está configurado)

**Pruebas**:
- Un solo aprobador, umbral 1 → aprobado después de 1
- Tres aprobadores, umbral 2 → aprobado después de 2
- Auto-aprobación bloqueada cuando está configurada
- Denegación falla inmediatamente (si está configurada)
- Ignorar comentarios de no-aprobadores

**Estado**: Completo

### Tareas
1. Definir tipos de aprobación (`internal/approval/types.go`):
   - ApprovalRequest, ApprovalStatus, Approval, Denial
2. Implementar analizador de comentarios (`internal/approval/parser.go`):
   - Extraer palabras clave de aprobación/denegación
   - Manejar variaciones (approve, approved, lgtm, /approve)
3. Implementar motor de grupo único (`internal/approval/engine.go`):
   - CollectApprovals de comentarios
   - EvaluateThreshold
   - CheckSelfApproval
4. Implementar seguimiento de estado (`internal/approval/status.go`):
   - Estados Pendiente/Aprobado/Denegado
   - Quién aprobó/denegó
5. Escribir pruebas del motor de aprobación

---

## Etapa 6: Motor de Aprobación - Lógica OR de Múltiples Grupos
**Objetivo**: Soportar múltiples grupos de requisitos con lógica OR
**Criterios de Éxito**:
- Múltiples grupos en el array `require:`
- Cualquier UN grupo que cumpla el umbral = aprobado
- Rastrea el estado por grupo de manera independiente
- Informa qué grupo fue satisfecho

**Pruebas**:
- Dos grupos: el primer grupo cumple el umbral → aprobado
- Dos grupos: el segundo grupo cumple el umbral → aprobado
- Dos grupos: ninguno cumple el umbral → pendiente
- Mixto: aprobadores en línea + referencia de política
- Anular min_approvals de política en el requisito

**Estado**: Completo

### Tareas
1. Ampliar motor para evaluación de múltiples grupos
2. Implementar resolución de políticas (búsqueda por nombre)
3. Añadir seguimiento de estado por grupo
4. Implementar lógica "gana el primer grupo satisfecho"
5. Añadir generación de nombre de requisito (nombre de política o hash)
6. Actualizar generación de tabla de estado para múltiples grupos
7. Escribir pruebas de múltiples grupos

---

## Etapa 7: Implementación de Acciones
**Objetivo**: Implementar puntos de entrada de acción (request, check, process-comment)
**Criterios de Éxito**:
- `request` crea issue de aprobación
- `check` devuelve el estado actual
- `process-comment` maneja comentarios de aprobación/denegación
- Todas las acciones establecen salidas adecuadas

**Pruebas**:
- Request crea issue con cuerpo correcto
- Check devuelve pendiente para nuevo issue
- Process-comment actualiza estado correctamente
- Las salidas coinciden con el formato esperado

**Estado**: Completo

### Tareas
1. Implementar acción de solicitud (`internal/action/request.go`):
   - Cargar configuración, resolver flujo de trabajo
   - Crear issue con tabla de estado
   - Establecer salidas (issue_number, issue_url)
2. Implementar acción de verificación (`internal/action/check.go`):
   - Cargar comentarios de issue
   - Evaluar estado de aprobación
   - Establecer salidas (estado, aprobadores)
3. Implementar acción de procesamiento de comentarios (`internal/action/process.go`):
   - Analizar comentario desencadenante
   - Actualizar estado de aprobación
   - Publicar comentario de actualización de estado
   - Crear etiqueta si está aprobado
   - Cerrar issue si está configurado
4. Implementar punto de entrada principal (`cmd/action/main.go`)
5. Escribir pruebas de estilo de integración

---

## Etapa 8: Integración de Equipos
**Objetivo**: Resolver miembros de equipo para aprobadores basados en equipo
**Criterios de Éxito**:
- Detectar formato `team:org/name` en aprobadores
- Resolver miembros de equipo a través de la API de GitHub
- Funciona con token de App de GitHub
- Error elegante cuando el token carece de permisos

**Pruebas**:
- Resolver team:org/engineers a lista de miembros
- Manejar error de equipo no encontrado
- Manejar error de permiso denegado
- Aprobadores mixtos de equipo + individuales

**Estado**: Completo

### Tareas
1. Añadir detección de equipo en análisis de aprobadores
2. Implementar resolución de miembros de equipo
3. Cachear miembros de equipo por solicitud (evitar llamadas repetidas a la API)
4. Añadir mensajes de error claros para problemas de autenticación
5. Documentar requisitos de token de App
6. Escribir pruebas de integración de equipo (simuladas)

---

## Etapa 9: Plantillas de Issues y UX
**Objetivo**: Generar issues de aprobación claros e informativos
**Criterios de Éxito**:
- La tabla de estado muestra todos los grupos con progreso
- Las variables de plantilla funcionan ({{version}}, etc.)
- Actualiza la tabla cuando cambia el estado
- Proporciona instrucciones claras de aprobación/denegación

**Pruebas**:
- Las variables de plantilla se reemplazan correctamente
- La tabla de estado se renderiza como se espera
- La tabla se actualiza en la aprobación
- El markdown se renderiza correctamente en GitHub

**Estado**: Completo

### Tareas
1. Crear plantilla de cuerpo de issue
2. Implementar sustitución de variables de plantilla
3. Crear generador de tabla de estado
4. Añadir lógica de actualización (editar cuerpo de issue al cambiar el estado)
5. Añadir marcador de estado oculto para seguimiento (JSON en comentario)
6. Probar renderizado de markdown

---

## Etapa 10: Pruebas de Extremo a Extremo y Pulido
**Objetivo**: Completar pruebas, documentación y preparación de lanzamiento
**Criterios de Éxito**:
- Todas las pruebas unitarias pasan
- Pruebas de integración con API de GitHub simulada
- Flujos de trabajo de ejemplo documentados
- README completo con ejemplos de uso
- La acción funciona en flujo de trabajo real de GitHub

**Pruebas**:
- Flujo completo: solicitud → aprobación → etiqueta creada
- Flujo completo: solicitud → denegación → cerrado
- Escenario de tiempo de espera
- Manejo de errores de configuración inválida
- Manejo de errores de permisos

**Estado**: Completo

### Tareas
1. Escribir pruebas E2E con simulaciones de API de GitHub
2. Crear flujos de trabajo de ejemplo en `examples/`
3. Escribir README exhaustivo
4. Crear guía de CONTRIBUTING
5. Configurar flujo de trabajo de lanzamiento (goreleaser)
6. Probar en repositorio real
7. Crear lanzamiento v1.0.0

---

## Decisiones Tecnológicas

### Lenguaje: Go
**Razonamiento**:
- Compila a un solo binario (inicio rápido de la acción)
- Tipado fuerte detecta errores de configuración en tiempo de análisis
- Excelentes bibliotecas de API de GitHub (`google/go-github`)
- Familiaridad del desarrollador

### Tipo de Acción: Docker
**Razonamiento**:
- Entorno consistente en todos los runners
- Sin dependencia de la instalación de Go del runner
- Mejor reproducibilidad
- Inicio ligeramente más lento, pero aceptable para flujos de trabajo de aprobación

### Configuración: YAML
**Razonamiento**:
- Familiar para usuarios de GitHub Actions
- Soporta comentarios para documentación
- Fácil de leer y editar
- El esquema JSON proporciona soporte en IDE

### Almacenamiento de Estado: Cuerpo de Issue + Comentarios
**Razonamiento**:
- Sin dependencias externas
- Registro completo de auditoría en GitHub
- Funciona con la interfaz nativa de GitHub
- Buscable y filtrable

---

## Dependencias

### Paquetes Go
```go
require (
    github.com/google/go-github/v57 v57.0.0
    github.com/sethvargo/go-githubactions v1.1.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/Masterminds/semver/v3 v3.2.1
    github.com/stretchr/testify v1.8.4
)
```

### Herramientas Externas
- Docker (para construir imagen de acción)
- goreleaser (para lanzamientos)
- golangci-lint (para linting)

---

## Mitigación de Riesgos

### Riesgo: Límites de Tasa de la API de GitHub
**Mitigación**:
- Usar solicitudes condicionales (ETags)
- Implementar retroceso exponencial
- Cachear resultados de membresía de equipo
- Recomendar GitHub App para límites más altos

### Riesgo: Expiración de Token (1 hora para tokens de App)
**Mitigación**:
- Documentar limitación claramente
- Sugerir patrón impulsado por eventos para aprobaciones largas
- Implementar tiempo de espera con mensajes claros

### Riesgo: Errores de Configuración Compleja
**Mitigación**:
- Esquema JSON para validación en IDE
- Mensajes de error claros con números de línea
- Validar configuración antes de cualquier operación
- Proporcionar ejemplos de configuración mínima

### Riesgo: Permisos de Membresía de Equipo
**Mitigación**:
- Documentación clara sobre requisitos de App
- Fallback elegante para usuarios individuales
- Error explícito cuando falla la búsqueda de equipo

---

## Etapa 11: Despliegue Progresivo de Pipelines
**Objetivo**: Seguimiento de un solo issue a través de múltiples entornos (dev → qa → stage → prod)
**Criterios de Éxito**:
- Un solo issue rastrea el despliegue a través de todas las etapas
- Cada etapa tiene su propia política de aprobación
- La tabla de progreso se actualiza a medida que las etapas son aprobadas
- El seguimiento de PR y commit muestra lo que se está desplegando
- Las etiquetas se crean en etapas configuradas

**Pruebas**:
- Issue de pipeline creado con todas las etapas pendientes
- Aprobar etapa 1 → avanza a etapa 2
- La tabla de progreso se actualiza correctamente
- El seguimiento de PR se llena desde el historial de git
- La etapa final cierra el issue

**Estado**: Completo

### Implementación

#### Archivos Creados/Modificados:
- `internal/action/pipeline.go` - Procesador de pipeline para gestión de etapas
- `internal/action/pipeline_template.go` - Generación de cuerpo de issue específico de pipeline
- `internal/github/commits.go` - APIs de comparación de Git y extracción de PR

#### Tipos Clave:
```go
// PipelineConfig en config/types.go
type PipelineConfig struct {
    Stages         []PipelineStage
    TrackPRs       bool
    TrackCommits   bool
    CompareFromTag string
    ReleaseStrategy ReleaseStrategyConfig
}

type PipelineStage struct {
    Name        string
    Environment string
    Policy      string
    Approvers   []string
    OnApproved  string
    CreateTag   bool
    IsFinal     bool
}
```

#### Funciones Clave:
- `PipelineProcessor.EvaluatePipelineStage()` - Evalúa la aprobación de la etapa actual
- `PipelineProcessor.ProcessPipelineApproval()` - Avanza el pipeline en la aprobación
- `GeneratePipelineIssueBody()` - Crea tabla de progreso con seguimiento de PR/commit
- `Client.GetMergedPRsBetween()` - Obtiene PRs entre dos referencias
- `Client.CompareCommits()` - Obtiene commits entre referencias

#### Ejemplo de Configuración:
```yaml
workflows:
  deploy:
    pipeline:
      track_prs: true
      track_commits: true
      stages:
        - name: dev
          policy: developers
          on_approved: "✅ DEV aprobado!"
        - name: qa
          policy: qa-team
        - name: prod
          policy: production-approvers
          create_tag: true
          is_final: true
```

#### Requisitos del Flujo de Trabajo:
- Permiso `pull-requests: read` para seguimiento de PR
- `contents: write` para creación de etiquetas
- `issues: write` para gestión de issues

---

## Etapa 12: Estrategias de Candidato a Lanzamiento
**Objetivo**: Soportar múltiples estrategias para seleccionar qué PRs pertenecen a un lanzamiento
**Criterios de Éxito**:
- Cuatro estrategias: etiqueta, rama, etiqueta, hito
- Creación automática del siguiente artefacto de lanzamiento al completar
- Limpieza opcional (cerrar hito, eliminar etiquetas, borrar rama)
- Soporte de flujo de trabajo de hotfix (omitir etapas)

**Pruebas**:
- Estrategia de etiqueta: PRs entre v1.0 y v2.0
- Estrategia de rama: PRs fusionados a release/v1.2.0
- Estrategia de etiqueta: PRs con etiqueta release:v1.2.0
- Estrategia de hito: PRs en hito v1.2.0
- Creación automática del siguiente hito al completar

**Estado**: Completo

### Implementación

#### Archivos Creados:
- `internal/config/release_strategy.go` - Tipos de configuración de estrategia
- `internal/github/releases.go` - API de GitHub para hitos, etiquetas, ramas
- `internal/action/release_tracker.go` - Rastreador de PR/commit consciente de estrategia

#### Tipos Clave:
```go
// ReleaseStrategyType enum
const (
    StrategyTag       ReleaseStrategyType = "tag"
    StrategyBranch    ReleaseStrategyType = "branch"
    StrategyLabel     ReleaseStrategyType = "label"
    StrategyMilestone ReleaseStrategyType = "milestone"
)

// ReleaseStrategyConfig en config/release_strategy.go
type ReleaseStrategyConfig struct {
    Type      ReleaseStrategyType
    Branch    BranchStrategyConfig
    Label     LabelStrategyConfig
    Milestone MilestoneStrategyConfig
    AutoCreate AutoCreateConfig
}

type AutoCreateConfig struct {
    Enabled     bool
    NextVersion string   // "patch", "minor", "major"
    CreateIssue bool
    Comment     string
}
```

#### Funciones Clave:
```go
// Métodos de ReleaseTracker
func (r *ReleaseTracker) GetReleaseContents(ctx, previousTag) (*ReleaseContents, error)
func (r *ReleaseTracker) CreateNextReleaseArtifact(ctx, nextVersion) error
func (r *ReleaseTracker) CleanupCurrentRelease(ctx, prs) error

// Métodos del cliente de GitHub
func (c *Client) GetPRsByMilestone(ctx, milestoneNumber) ([]PullRequest, error)
func (c *Client) GetPRsByLabel(ctx, label) ([]PullRequest, error)
func (c *Client) GetPRsMergedToBranch(ctx, branchName) ([]PullRequest, error)
func (c *Client) CreateMilestone(ctx, title, description) (*Milestone, error)
func (c *Client) CreateBranch(ctx, branchName, sourceRef) (*Branch, error)
func (c *Client) CreateLabel(ctx, name, color, description) error
```

#### Ejemplos de Configuración:

**Estrategia de Hito:**
```yaml
pipeline:
  release_strategy:
    type: milestone
    milestone:
      pattern: "v{{version}}"
      close_after_release: true
    auto_create:
      enabled: true
      next_version: minor
      create_issue: true
```

**Estrategia de Rama:**
```yaml
pipeline:
  release_strategy:
    type: branch
    branch:
      pattern: "release/{{version}}"
      base_branch: main
      delete_after_release: false
```

**Estrategia de Etiqueta:**
```yaml
pipeline:
  release_strategy:
    type: label
    label:
      pattern: "release:{{version}}"
      pending_label: "pending-release"
      remove_after_release: true
```

**Flujo de Trabajo de Hotfix (flujo separado, estrategia de etiqueta):**
```yaml
workflows:
  hotfix:
    description: "Hotfix de emergencia - directo a prod"
    pipeline:
      release_strategy:
        type: tag   # Sin limpieza, sin creación automática
      stages:
        - name: prod
          policy: production-approvers
          create_tag: true
          is_final: true
```

#### Opciones de Limpieza (todas por defecto en false):
| Estrategia | Opción | Descripción |
|------------|--------|-------------|
| Rama | `delete_after_release` | Borrar rama de lanzamiento |
| Etiqueta | `remove_after_release` | Eliminar etiquetas de PRs |
| Hito | `close_after_release` | Cerrar el hito |

#### Flujo de Creación Automática:
1. Etapa final (prod) aprobada
2. Calcular siguiente versión (patch/minor/major)
3. Crear siguiente artefacto (rama/etiqueta/hito)
4. Opcionalmente crear nuevo issue de aprobación
5. Publicar comentario sobre el próximo lanzamiento

---

## Etapa 13: Visualización de Pipeline (Diagramas Mermaid)
**Objetivo**: Añadir diagramas de flujo visuales a los issues de aprobación de pipeline
**Criterios de Éxito**:
- Diagrama Mermaid muestra etapas del pipeline con nodos coloreados
- Los colores se actualizan según el estado de la etapa (completado, actual, pendiente, auto-aprobado)
- Puede desactivarse mediante configuración

**Pruebas**:
- Generar diagrama con todas las etapas pendientes
- Generar diagrama con etapas completadas
- Generar diagrama con etapas auto-aprobadas
- Generar diagrama cuando está desactivado (devuelve cadena vacía)

**Estado**: Completo

### Implementación

#### Archivos Modificados:
- `internal/action/pipeline.go` - Añadida función `GeneratePipelineMermaid()`
- `internal/action/template.go` - Añadido campo `PipelineMermaid` a `TemplateData`
- `internal/config/types.go` - Añadida opción `ShowMermaidDiagram` a `PipelineConfig`

#### Funciones Clave:
```go
// GeneratePipelineMermaid genera un diagrama de flujo Mermaid para el pipeline
func GeneratePipelineMermaid(state *IssueState, pipeline *config.PipelineConfig) string

// ShouldShowMermaidDiagram devuelve si mostrar el diagrama (por defecto: true)
func (p *PipelineConfig) ShouldShowMermaidDiagram() bool
```

#### Esquema de Colores:
| Estado | Color | Código Hex |
|--------|-------|------------|
| Completado | Verde | `#28a745` |
| Actual | Amarillo/Ámbar | `#ffc107` |
| Pendiente | Gris | `#6c757d` |
| Auto-aprobado | Cian | `#17a2b8` |

#### Emojis en Etiquetas:
- ✅ - Completado (aprobación manual)
- 🤖 - Auto-aprobado o auto-aprobación pendiente
- ⏳ - Etapa actual esperando aprobación
- ⬜ - Etapas futuras pendientes

#### Configuración:
```yaml
pipeline:
  show_mermaid_diagram: true  # Por defecto: true
  stages:
    - name: dev
    - name: prod
```

---

## Etapa 14: Experiencia de Usuario de Aprobación Mejorada (Sub-Issues y Comentarios)
**Objetivo**: Proporcionar experiencias de aprobación interactivas a través de sub-issues y comentarios mejorados
**Criterios de Éxito**:
- Sub-issues creados para cada etapa del pipeline cuando está configurado
- Cerrar sub-issue = aprobar la etapa
- Reacciones de emoji en comentarios de aprobación/denegación
- Sección de Acciones Rápidas en el cuerpo del issue
- Protección de cierre de issue (reabrir cierres no autorizados)
- Modo híbrido: mezclar comentarios y sub-issues por etapa

**Pruebas**:
- Crear pipeline con modo de sub-issues → sub-issues creados y vinculados
- Cerrar sub-issue → etapa aprobada, issue padre actualizado
- Cierre no autorizado → issue reabierto con advertencia
- Modo híbrido respeta sobrescrituras por etapa
- Reacciones de comentarios añadidas en aprobación/denegación

**Estado**: Completo

### Implementación

#### Fase 1: UX de Comentarios Mejorada
- **Reacciones de emoji** en comentarios de aprobación: 👍 aprobado, 👎 denegado, 👀 visto
- **Sección de Acciones Rápidas** en el cuerpo del issue con tabla de referencia de comandos
- **Configuración** a través de `comment_settings` en el flujo de trabajo

#### Fase 2: Sub-Issues para Aprobaciones
- **Modos de aprobación**: `comments` (por defecto), `sub_issues`, `hybrid`
- **Configuraciones de sub-issue**: plantillas de título/cuerpo, etiquetas, protección
- **Sobrescritura por etapa**: `approval_mode` en etapas individuales
- **Protección de cierre**: reabrir automáticamente si es cerrado por usuario no autorizado
- **Protección de padre**: prevenir cierre de padre hasta que sub-issues estén completados

#### Archivos Creados/Modificados:
- `internal/action/sub_issue_handler.go` - Creación y manejo de cierre de sub-issues
- `internal/action/action.go` - Soporte de reacciones, manejador `ProcessSubIssueClose`
- `internal/action/pipeline.go` - `GeneratePipelineIssueBodyWithSubIssues()`
- `internal/action/template.go` - Estructura `SubIssueInfo` en `IssueState`
- `internal/config/types.go` - `ApprovalMode`, `SubIssueSettings`, `CommentSettings`
- `internal/github/issues.go` - `GetIssueByNumber`, `ReopenIssue`
- `internal/github/sub_issues.go` - Envoltorio de API de Sub-Issues de GitHub

#### Tipos Clave:
```go
// ApprovalMode define cómo se recogen las aprobaciones
type ApprovalMode string
const (
    ApprovalModeComments  ApprovalMode = "comments"
    ApprovalModeSubIssues ApprovalMode = "sub_issues"
    ApprovalModeHybrid    ApprovalMode = "hybrid"
)

// SubIssueSettings configura la UX de aprobación basada en sub-issues
type SubIssueSettings struct {
    TitleTemplate      string
    BodyTemplate       string
    Labels             []string
    AutoCloseRemaining bool
    Protection         *SubIssueProtection
}

// SubIssueProtection configura la protección de cierre de issue
type SubIssueProtection struct {
    OnlyAssigneeCanClose   bool
    RequireApprovalComment bool
    PreventParentClose     bool
}

// CommentSettings configura la UX de comentarios mejorada
type CommentSettings struct {
    ReactToComments    *bool
    ShowQuickActions   *bool
    RequireSlashPrefix bool
}
```

#### Ejemplo de Configuración:
```yaml
workflows:
  deploy:
    approval_mode: sub_issues
    sub_issue_settings:
      title_template: "⏳ Aprobar: {{stage}} para {{version}}"  # ✅ cuando aprobado
      labels: [approval-stage]
      protection:
        only_assignee_can_close: true
        prevent_parent_close: true
    comment_settings:
      react_to_comments: true
      show_quick_actions: true
    pipeline:
      stages:
        - name: dev
          policy: dev-team
        - name: prod
          policy: prod-team
          approval_mode: sub_issues  # Sobrescritura por etapa
```

---

## Mejoras Futuras

### Características Planeadas
- **Integración con Slack/Teams**: Notificar a canales sobre solicitudes de aprobación
- **Lanzamientos Programados**: Ventanas de lanzamiento basadas en tiempo
- **Flujos de Trabajo de Reversión**: Reversión con un clic con aprobación
- **Panel de Métricas**: Tiempo de ciclo de aprobación, análisis de cuellos de botella
- **Lanzamientos Multi-Repo**: Coordinar lanzamientos entre repositorios

### Extensiones de API
- Soporte de webhook para integraciones externas
- API REST para acceso programático
- Consultas GraphQL para verificaciones de estado complejas