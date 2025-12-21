# Motor de Aprobación Empresarial

GitHub Action de nivel empresarial para flujos de trabajo de aprobación basados en políticas con umbrales por grupo (X de N), lógica OR entre grupos y creación automática de etiquetas semver.

## Características

- **Lógica de Aprobación Flexible**: Soporte para lógica AND (todos deben aprobar) y umbral (X de N) dentro de los grupos.
- **Lógica OR Entre Grupos**: Múltiples caminos de aprobación: cualquier grupo que cumpla los requisitos aprueba la solicitud.
- **Aprobadores Mixtos**: Combina usuarios individuales y equipos de GitHub en el mismo grupo.
- **Pipelines de Despliegue Progresivo**: Seguimiento de un solo problema a través de múltiples entornos (dev → qa → stage → prod).
- **Visualización del Pipeline**: Diagramas de flujo codificados por colores en Mermaid que muestran el progreso del despliegue.
- **Aprobaciones de Sub-Issue**: Crea sub-issues de aprobación dedicados para cada etapa: cerrar para aprobar.
- **Experiencia de Usuario Mejorada en Comentarios**: Reacciones de emoji en comentarios de aprobación, sección de Acciones Rápidas con referencia de comandos.
- **Protección de Cierre de Issues**: Evita que usuarios no autorizados cierren issues de aprobación (reapertura automática).
- **Modos de Aprobación Híbridos**: Mezcla aprobaciones basadas en comentarios y sub-issues por flujo de trabajo o etapa.
- **Seguimiento de PR y Commits**: Lista automáticamente PRs y commits en issues de despliegue para la gestión de lanzamientos.
- **Creación de Etiquetas Semver**: Crea automáticamente etiquetas git tras la aprobación.
- **Configuración Basada en Políticas**: Define políticas de aprobación reutilizables en YAML.
- **Flujo de Trabajo Basado en Issues**: Rastro de auditoría transparente a través de issues de GitHub.
- **Integración con Jira**: Extrae claves de issues de commits, muestra en issues de aprobación, actualiza Fix Versions.
- **Seguimiento de Despliegue**: Crea despliegues de GitHub para visibilidad en el panel de despliegue.
- **Configuración Externa**: Centraliza políticas de aprobación en un repositorio compartido.
- **Manejo de Límites de Tasa**: Reintento automático con retroceso exponencial para límites de tasa de la API de GitHub.
- **Servidor Empresarial de GitHub**: Soporte completo para entornos GHES.
- **Sin Dependencias Externas**: Acciones puras de GitHub, no se requieren servicios externos.

## Tabla de Contenidos

- [Inicio Rápido](#inicio-rápido)
- [Referencia de Acción](#referencia-de-acción)
  - [Acciones](#acciones)
  - [Entradas](#entradas)
  - [Salidas](#salidas)
- [Referencia de Configuración](#referencia-de-configuración)
  - [Políticas](#políticas)
  - [Flujos de Trabajo](#flujos-de-trabajo)
  - [Etiquetado](#configuración-de-etiquetado)
  - [Plantillas Personalizadas](#plantillas-de-issues-personalizadas)
  - [Valores Predeterminados](#valores-predeterminados)
  - [Semver](#semver)
- [Referencia Completa de Configuración](#referencia-completa-de-configuración)
- [Detalles de las Características](#detalles-de-las-características)
  - [Palabras Clave de Aprobación](#palabras-clave-de-aprobación)
  - [Soporte de Equipos](#soporte-de-equipos)
  - [Pipelines de Despliegue Progresivo](#pipelines-de-despliegue-progresivo)
  - [Estrategias de Candidatos a Lanzamiento](#estrategias-de-candidatos-a-lanzamiento)
  - [Integración con Jira](#integración-con-jira)
  - [Seguimiento de Despliegue](#seguimiento-de-despliegue)
  - [Repositorio de Configuración Externa](#repositorio-de-configuración-externa)
  - [Aprobaciones de Bloqueo](#aprobaciones-de-bloqueo)
  - [Eliminación de Etiquetas](#eliminación-de-etiquetas-al-cerrar-issue)
- [Ejemplos Completos](#ejemplos-completos)
- [Validación de Esquema](#validación-de-esquema)
- [Servidor Empresarial de GitHub](#servidor-empresarial-de-github)

## Inicio Rápido

### 1. Crear Configuración

Crea `.github/approvals.yml` en tu repositorio:

```yaml
version: 1

policies:
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2

  platform-team:
    approvers: [team:platform-engineers]
    require_all: true

workflows:
  production-deploy:
    require:
      # Lógica OR: cualquier camino satisface la aprobación
      - policy: dev-team        # 2 de 3 desarrolladores
      - policy: platform-team   # TODOS los ingenieros de plataforma
    on_approved:
      create_tag: true
      close_issue: true
```

### 2. Solicitar Flujo de Trabajo de Aprobación

Crea `.github/workflows/request-approval.yml`:

```yaml
name: Solicitar Aprobación de Despliegue

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Versión a desplegar (por ejemplo, v1.2.3)'
        required: true
        type: string

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        id: approval
        with:
          action: request
          workflow: production-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Output Results
        run: |
          echo "Issue: ${{ steps.approval.outputs.issue_url }}"
          echo "Status: ${{ steps.approval.outputs.status }}"
```

### 3. Manejar Comentarios de Aprobación

Crea `.github/workflows/handle-approval.yml`:

```yaml
name: Manejar Comentarios de Aprobación

on:
  issue_comment:
    types: [created]

jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        id: process
        with:
          action: process-comment
          issue_number: ${{ github.event.issue.number }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Trigger Deployment
        if: steps.process.outputs.status == 'approved'
        run: |
          echo "Approved by: ${{ steps.process.outputs.approvers }}"
          echo "Tag created: ${{ steps.process.outputs.tag }}"
```

---

## Referencia de Acción

### Acciones

La acción soporta cuatro modos de operación a través de la entrada `action`:

| Acción | Descripción | Cuándo Usar |
|--------|-------------|-------------|
| `request` | Crear un nuevo issue de solicitud de aprobación | Al iniciar un flujo de trabajo de despliegue/lanzamiento |
| `process-comment` | Procesar un comentario de aprobación/denegación | En eventos `issue_comment` |
| `check` | Verificar el estado actual de aprobación | Para sondear la finalización de la aprobación |
| `close-issue` | Manejar eventos de cierre de issues | En eventos `issues: [closed]` |

### Entradas

#### Entradas Principales

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `action` | Acción a realizar: `request`, `check`, `process-comment`, `close-issue` | Sí | - |
| `workflow` | Nombre del flujo de trabajo desde la configuración (para acción `request`) | Para `request` | - |
| `version` | Versión semver para la creación de etiquetas (por ejemplo, `1.2.3` o `v1.2.3`) | No | - |
| `issue_number` | Número de issue (para `check`, `process-comment`, `close-issue`) | Para check/process/close | - |
| `token` | Token de GitHub para operaciones de API | Sí | - |

#### Entradas de Configuración

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `config_path` | Ruta al archivo de configuración approvals.yml | No | `.github/approvals.yml` |
| `config_repo` | Repositorio externo para configuración compartida (por ejemplo, `org/.github`) | No | - |

#### Entradas de Sondeo (para acción `check`)

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `wait` | Esperar aprobación (sondeo) en lugar de devolver inmediatamente | No | `false` |
| `timeout` | Tiempo de espera para esperar (por ejemplo, `24h`, `1h30m`, `30m`) | No | `72h` |

#### Entradas de Soporte de Equipos

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `app_id` | ID de la App de GitHub para verificaciones de membresía de equipo | No | - |
| `app_private_key` | Clave privada de la App de GitHub para verificaciones de membresía de equipo | No | - |

#### Entradas de Integración con Jira

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `jira_base_url` | URL base de Jira Cloud (por ejemplo, `https://yourcompany.atlassian.net`) | No | - |
| `jira_user_email` | Correo electrónico del usuario de Jira para autenticación de API | No | - |
| `jira_api_token` | Token de API de Jira para autenticación | No | - |
| `jira_update_fix_version` | Actualizar issues de Jira con Fix Version al aprobar | No | `true` |
| `include_jira_issues` | Incluir issues de Jira en el cuerpo de la solicitud de aprobación | No | `true` |

#### Entradas de Seguimiento de Despliegue

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `create_deployment` | Crear despliegue de GitHub para seguimiento | No | `true` |
| `deployment_environment` | Entorno objetivo (por ejemplo, `production`, `staging`) | No | `production` |
| `deployment_environment_url` | URL al entorno desplegado | No | - |

#### Otras Entradas

| Entrada | Descripción | Requerido | Predeterminado |
|---------|-------------|-----------|---------------|
| `issue_action` | Acción de evento de issue para `close-issue` (`closed`, `reopened`) | No | - |
| `previous_tag` | Etiqueta anterior para comparar commits (detectada automáticamente si no se especifica) | No | - |

### Salidas

#### Salidas Principales

| Salida | Descripción | Disponible Para |
|--------|-------------|-----------------|
| `status` | Estado de aprobación: `pending`, `approved`, `denied`, `timeout`, `tag_deleted`, `skipped` | Todas las acciones |
| `issue_number` | Número de issue creado o verificado | Todas las acciones |
| `issue_url` | URL al issue de aprobación | Todas las acciones |

#### Salidas de Aprobación

| Salida | Descripción | Disponible Para |
|--------|-------------|-----------------|
| `approvers` | Lista separada por comas de usuarios que aprobaron | `process-comment`, `check` |
| `denier` | Usuario que denegó la solicitud | `process-comment`, `check` |
| `satisfied_group` | Nombre del grupo que satisfizo la aprobación | `process-comment`, `check` |
| `tag` | Nombre de la etiqueta creada | `process-comment` (al aprobar) |
| `tag_deleted` | Etiqueta que fue eliminada | `close-issue` |

#### Salidas de Jira

| Salida | Descripción | Disponible Para |
|--------|-------------|-----------------|
| `jira_issues` | Lista separada por comas de claves de issues de Jira en este lanzamiento | `request` |
| `jira_issues_json` | Array JSON de detalles de issues de Jira (clave, resumen, tipo, estado) | `request` |

#### Salidas de Despliegue

| Salida | Descripción | Disponible Para |
|--------|-------------|-----------------|
| `deployment_id` | ID de despliegue de GitHub para actualizaciones de estado | `request` |
| `deployment_url` | URL al despliegue en GitHub | `request` |

#### Salidas de Notas de Lanzamiento

| Salida | Descripción | Disponible Para |
|--------|-------------|-----------------|
| `release_notes` | Notas de lanzamiento generadas automáticamente a partir de commits e issues de Jira | `request` |
| `commits_count` | Número de commits en este lanzamiento | `request` |

---

## Referencia de Configuración

### Políticas

Las políticas definen grupos reutilizables de aprobadores. Hay dos formatos:

#### Formato Simple

```yaml
policies:
  # Basado en umbral: X de N deben aprobar
  dev-team:
    approvers: [alice, bob, charlie]
    min_approvals: 2

  # Todos deben aprobar (lógica AND)
  security:
    approvers: [team:security, security-lead]
    require_all: true

  # Equipos y personas mezclados
  production:
    approvers:
      - team:sre
      - tech-lead
      - product-owner
    min_approvals: 2
```

#### Formato Avanzado (umbrales por fuente)

Para requisitos complejos como "2 de plataforma Y 1 de seguridad":

```yaml
policies:
  # Puerta AND compleja
  production-gate:
    from:
      - team: platform-engineers
        min_approvals: 2        # 2 del equipo de plataforma
      - team: security
        min_approvals: 1        # 1 del equipo de seguridad
      - user: alice             # alice también debe aprobar
    logic: and                  # TODAS las fuentes deben ser satisfechas

  # Puerta OR flexible
  flexible-review:
    from:
      - team: security
        require_all: true       # Todo el equipo de seguridad
      - team: platform
        min_approvals: 2        # O 2 miembros de plataforma
    logic: or                   # CUALQUIER fuente es suficiente

  # Aprobación ejecutiva: cualquier ejecutivo
  exec-approval:
    from:
      - user: ceo
      - user: cto
      - user: vp-engineering
    logic: or

  # Lista de usuarios con umbral
  leads:
    from:
      - users: [tech-lead, product-lead, design-lead]
        min_approvals: 2
```

**Tipos de fuente:**

- `team: slug` - Equipo de GitHub (requiere token de App)
- `user: username` - Usuario único (require_all implícito)
- `users: [a, b, c]` - Lista de usuarios

**Lógica a nivel de política:**

- `logic: and` - TODAS las fuentes deben ser satisfechas (predeterminado)
- `logic: or` - CUALQUIER fuente satisfecha es suficiente

#### Lógica en Línea (mezcla AND/OR)

Para expresiones complejas, usa `logic:` en cada fuente para especificar cómo se conecta a la siguiente:

```yaml
policies:
  # (2 de seguridad Y 2 de plataforma) O alice
  complex-gate:
    from:
      - team: security
        min_approvals: 2
        logic: and              # Y con la siguiente fuente
      - team: platform
        min_approvals: 2
        logic: or               # O con la siguiente fuente
      - user: alice            # alice sola puede satisfacer

  # (seguridad Y plataforma) O (alice Y bob) O manager
  multi-path:
    from:
      - team: security
        min_approvals: 1
        logic: and
      - team: platform
        min_approvals: 1
        logic: or               # Fin del primer grupo AND
      - user: alice
        logic: and
      - user: bob
        logic: or               # Fin del segundo grupo AND
      - user: manager          # Tercer camino
```

**Precedencia de operadores:** AND se une más fuerte que OR (lógica booleana estándar).

La expresión `A and B or C and D` se evalúa como `(A AND B) OR (C AND D)`.

### Flujos de Trabajo

Los flujos de trabajo definen requisitos de aprobación y acciones:

```yaml
workflows:
  my-workflow:
    description: "Descripción opcional"

    # Condiciones de activación (para filtrado)
    trigger:
      environment: production

    # Requisitos de aprobación (lógica OR entre elementos)
    require:
      - policy: dev-team
      - policy: security
      # O aprobadores en línea:
      - approvers: [alice, bob]
        require_all: true

    # Configuración de issues
    issue:
      title: "Aprobación: {{version}}"
      body: |                          # Plantilla personalizada en línea (opcional)
        ## Mi Issue de Aprobación Personalizado
        Versión: {{.Version}}
        Solicitado por: @{{.Requestor}}
        {{.GroupsTable}}
      body_file: "templates/my-template.md"  # O cargar desde archivo
      labels: [production, deploy]
      assignees_from_policy: true

    # Acciones al aprobar
    on_approved:
      create_tag: true
      tag_prefix: "v"  # Crea v1.2.3
      close_issue: true
      comment: "¡Aprobado! Etiqueta {{version}} creada."

    # Acciones al denegar
    on_denied:
      close_issue: true
      comment: "Denegado por {{denier}}."

    # Acciones cuando el issue se cierra manualmente
    on_closed:
      delete_tag: true   # Eliminar la etiqueta si el issue se cierra
      comment: "Despliegue cancelado. Etiqueta {{tag}} eliminada."
```

### Configuración de Etiquetado

Controla cómo se crean las etiquetas por flujo de trabajo:

```yaml
workflows:
  dev-deploy:
    require:
      - policy: dev-team
    on_approved:
      tagging:
        enabled: true
        start_version: "0.1.0"      # Sin prefijo 'v', comienza en 0.1.0
        auto_increment: patch        # Incremento automático: 0.1.0 -> 0.1.1 -> 0.1.2
        env_prefix: "dev-"           # Crea: dev-0.1.0, dev-0.1.1

  staging-deploy:
    require:
      - policy: qa-team
    on_approved:
      tagging:
        enabled: true
        start_version: "v1.0.0"     # Prefijo 'v' (inferido de start_version)
        auto_increment: minor        # v1.0.0 -> v1.1.0 -> v1.2.0
        env_prefix: "staging-"       # Crea: staging-v1.0.0

  production-deploy:
    require:
      - policy: prod-team
    on_approved:
      tagging:
        enabled: true
        start_version: "v1.0.0"     # Se requiere versión manual (sin auto_increment)
```

**Opciones de etiquetado:**

| Opción | Descripción |
|--------|-------------|
| `enabled` | Habilitar creación de etiquetas |
| `start_version` | Versión inicial y formato (por ejemplo, "v1.0.0" o "1.0.0") |
| `prefix` | Prefijo de versión (inferido de `start_version` si no se establece) |
| `auto_increment` | Incremento automático: `major`, `minor`, `patch`, o omitir para manual |
| `env_prefix` | Prefijo de entorno (por ejemplo, "dev-" crea "dev-v1.0.0") |

### Plantillas de Issues Personalizadas

Puedes personalizar completamente el cuerpo del issue usando plantillas Go. Usa `body` para plantillas en línea o `body_file` para cargar desde un archivo.

**Variables de plantilla disponibles:**

| Variable | Descripción |
|----------|-------------|
| `{{.Title}}` | Título del issue |
| `{{.Description}}` | Descripción del flujo de trabajo |
| `{{.Version}}` | Versión semver |
| `{{.Requestor}}` | Nombre de usuario de GitHub que solicitó |
| `{{.Environment}}` | Nombre del entorno |
| `{{.RunURL}}` | Enlace a la ejecución del flujo de trabajo |
| `{{.RepoURL}}` | URL del repositorio |
| `{{.CommitSHA}}` | SHA completo del commit |
| `{{.CommitURL}}` | Enlace al commit |
| `{{.Branch}}` | Nombre de la rama |
| `{{.GroupsTable}}` | Tabla de estado de aprobación pre-renderizada |
| `{{.Timestamp}}` | Marca de tiempo de la solicitud |
| `{{.PreviousVersion}}` | Versión/etiqueta anterior |
| `{{.CommitsCount}}` | Número de commits en este lanzamiento |
| `{{.HasJiraIssues}}` | Booleano - si existen issues de Jira |
| `{{.JiraIssues}}` | Array de datos de issues de Jira |
| `{{.JiraIssuesTable}}` | Tabla de issues de Jira pre-renderizada |
| `{{.PipelineTable}}` | Tabla de pipeline de despliegue pre-renderizada |
| `{{.PipelineMermaid}}` | Diagrama de flujo de Mermaid pre-renderizado |
| `{{.Vars.key}}` | Variables personalizadas |

**Funciones de plantilla:**

| Función | Ejemplo | Descripción |
|---------|---------|-------------|
| `slice` | `{{slice .CommitSHA 0 7}}` | Subcadena (SHA corto) |
| `title` | `{{.Environment \| title}}` | Título en mayúsculas |
| `upper` | `{{.Version \| upper}}` | Mayúsculas |
| `lower` | `{{.Version \| lower}}` | Minúsculas |
| `join` | `{{join .Groups ","}}` | Unir array |
| `contains` | `{{if contains .Branch "feature"}}` | Verificar subcadena |
| `replace` | `{{replace .Version "v" ""}}` | Reemplazar cadena |
| `default` | `{{default "N/A" .Environment}}` | Valor predeterminado |

**Ejemplo de archivo de plantilla personalizada** (`.github/templates/deploy.md`):

```markdown
## {{.Title}}

### Información del Lanzamiento

- **Versión:** `{{.Version}}`
- **Solicitado por:** @{{.Requestor}}
{{- if .CommitSHA}}
- **Commit:** [{{slice .CommitSHA 0 7}}]({{.CommitURL}})
{{- end}}
{{- if .CommitsCount}}
- **Cambios:** {{.CommitsCount}} commits desde {{.PreviousVersion}}
{{- end}}

{{if .HasJiraIssues}}
### Issues de Jira

{{.JiraIssuesTable}}
{{end}}

### Estado de Aprobación

{{.GroupsTable}}

---

**Aprobar:** Comentar `approve` | **Denegar:** Comentar `deny`
```

### Valores Predeterminados

Valores predeterminados globales que se aplican a todos los flujos de trabajo:

```yaml
defaults:
  timeout: 72h                    # Tiempo de espera predeterminado para aprobación
  allow_self_approval: false      # Si los solicitantes pueden aprobar sus propias solicitudes
  issue_labels:                   # Etiquetas añadidas a todos los issues de aprobación
    - approval-required
```

### Semver

Configura el manejo de versiones:

```yaml
semver:
  prefix: "v"              # Prefijo de etiqueta (v1.2.3)
  strategy: input          # Usar versión de la entrada
  validate: true           # Validar formato semver
  allow_prerelease: true   # Permitir versiones preliminares (por ejemplo, v1.0.0-beta.1)
  auto:                    # Incremento automático basado en etiquetas (cuando strategy: auto)
    major_labels: [breaking, major]
    minor_labels: [feature, minor]
    patch_labels: [fix, patch, bug]
```

---

## Referencia Completa de Configuración

Esta sección documenta **cada opción de configuración** disponible en `approvals.yml`.

### Estructura de Nivel Superior

```yaml
version: 1                    # Requerido: versión de configuración (siempre 1)
defaults: { ... }             # Opcional: valores predeterminados globales
policies: { ... }             # Requerido: políticas de aprobación reutilizables
workflows: { ... }            # Requerido: flujos de trabajo de aprobación
semver: { ... }               # Opcional: configuraciones de manejo de versiones
```

### Opciones de `defaults`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `timeout` | duración | `72h` | Tiempo de espera para la acción de `check` bloqueante con `wait: true`. Usa horas (por ejemplo, `168h` para 1 semana). No necesario para flujos de trabajo basados en eventos. |
| `allow_self_approval` | bool | `false` | Si el solicitante puede aprobar su propia solicitud |
| `issue_labels` | string[] | `[]` | Etiquetas añadidas a todos los issues de aprobación |

### Opciones de `policies.<name>` (Formato Simple)

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `approvers` | string[] | - | Lista de nombres de usuario o referencias `team:slug` |
| `min_approvals` | int | 0 | Número de aprobaciones requeridas (0 = usar `require_all`) |
| `require_all` | bool | `false` | Si es verdadero, TODOS los aprobadores deben aprobar |

### Opciones de `policies.<name>` (Formato Avanzado)

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `from` | source[] | - | Lista de fuentes de aprobadores con umbrales individuales |
| `logic` | string | `"and"` | Cómo combinar fuentes: `"and"` o `"or"` |

**Opciones de Fuente de Aprobadores (`from[]`):**

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `team` | string | - | Slug del equipo (por ejemplo, `"platform"` o `"org/platform"`) |
| `user` | string | - | Nombre de usuario único |
| `users` | string[] | - | Lista de nombres de usuario |
| `min_approvals` | int | 1 | Aprobaciones requeridas de esta fuente |
| `require_all` | bool | `false` | Todos de esta fuente deben aprobar |
| `logic` | string | - | Lógica para la siguiente fuente: `"and"` o `"or"` |

### Opciones de `workflows.<name>`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `description` | string | - | Descripción legible por humanos |
| `trigger` | map | - | Condiciones de activación (para filtrado) |
| `require` | requirement[] | - | **Requerido:** Requisitos de aprobación (lógica OR entre elementos) |
| `issue` | object | - | Configuración de creación de issues |
| `on_approved` | object | - | Acciones al aprobar |
| `on_denied` | object | - | Acciones al denegar |
| `on_closed` | object | - | Acciones cuando el issue se cierra manualmente |
| `pipeline` | object | - | Configuración de pipeline de despliegue progresivo |

### Opciones de `workflows.<name>.require[]`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `policy` | string | - | Referencia a una política definida |
| `approvers` | string[] | - | Aprobadores en línea (alternativa a la política) |
| `min_approvals` | int | - | Sobrescribir min_approvals de la política |
| `require_all` | bool | - | Sobrescribir require_all de la política |

### Opciones de `workflows.<name>.issue`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `title` | string | `"Aprobación Requerida: {workflow}"` | Título del issue (soporta `{{version}}`, `{{environment}}`, `{{workflow}}`) |
| `body` | string | - | Plantilla de cuerpo de issue personalizada (sintaxis de plantilla Go) |
| `body_file` | string | - | Ruta al archivo de plantilla (relativa a `.github/`) |
| `labels` | string[] | `[]` | Etiquetas adicionales para este flujo de trabajo |
| `assignees_from_policy` | bool | `false` | Asignar automáticamente usuarios individuales de políticas (máximo 10) |

### Opciones de `workflows.<name>.on_approved`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `create_tag` | bool | `false` | Crear una etiqueta git (usa la versión de entrada) |
| `close_issue` | bool | `false` | Cerrar el issue después de la aprobación |
| `comment` | string | - | Comentario a publicar (soporta `{{version}}`, `{{satisfied_group}}`) |
| `tagging` | object | - | Configuración avanzada de etiquetado |

### Opciones de `workflows.<name>.on_approved.tagging`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `enabled` | bool | `false` | Habilitar creación de etiquetas |
| `start_version` | string | `"0.0.0"` | Versión inicial (por ejemplo, `"v1.0.0"` o `"1.0.0"`) |
| `prefix` | string | (inferido) | Prefijo de versión (inferido de `start_version`) |
| `auto_increment` | string | - | Incremento automático: `"major"`, `"minor"`, `"patch"`, o omitir para manual |
| `env_prefix` | string | - | Prefijo de entorno (por ejemplo, `"dev-"` crea `"dev-v1.0.0"`) |

### Opciones de `workflows.<name>.on_denied`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `close_issue` | bool | `false` | Cerrar el issue después de la denegación |
| `comment` | string | - | Comentario a publicar (soporta `{{denier}}`) |

### Opciones de `workflows.<name>.on_closed`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|----------------|-------------|
| `delete_tag` | bool | `false` | Eliminar la etiqueta asociada cuando el issue se cierra |
| `comment` | string | - | Comentario a publicar (soporta `{{tag}}`, `{{version}}`) |

### Opciones de `workflows.<name>.pipeline`

Esta sección está destinada a proporcionar una guía completa sobre cómo configurar y utilizar el Motor de Aprobación Empresarial en GitHub, permitiendo a las organizaciones gestionar de manera eficiente los flujos de trabajo de aprobación basados en políticas.

Por supuesto, aquí tienes la traducción al español del texto proporcionado:

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `stages` | stage[] | - | **Requerido:** Lista ordenada de etapas de despliegue |
| `track_prs` | bool | `false` | Incluir PRs fusionados en el cuerpo del issue |
| `track_commits` | bool | `false` | Incluir commits en el cuerpo del issue |
| `compare_from_tag` | string | - | Patrón de etiqueta para comparar desde (por ejemplo, `"v*"`) |
| `show_mermaid_diagram` | bool | `true` | Mostrar diagrama de flujo visual de Mermaid de las etapas de la tubería |
| `release_strategy` | object | - | Estrategia de selección de candidato de lanzamiento |

### Opciones de `workflows.<nombre>.pipeline.stages[]`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `name` | string | - | **Requerido:** Nombre de la etapa (por ejemplo, `"dev"`, `"prod"`) |
| `environment` | string | - | Nombre del entorno de GitHub |
| `policy` | string | - | Política de aprobación para esta etapa |
| `approvers` | string[] | - | Aprobadores en línea (alternativa a la política) |
| `on_approved` | string | - | Comentario a publicar cuando la etapa es aprobada |
| `create_tag` | bool | `false` | Crear una etiqueta git en esta etapa |
| `is_final` | bool | `false` | Cerrar issue después de esta etapa |
| `auto_approve` | bool | `false` | Aprobar automáticamente sin intervención humana |

### Opciones de `workflows.<nombre>.pipeline.release_strategy`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `type` | string | `"tag"` | Estrategia: `"tag"`, `"branch"`, `"label"`, `"milestone"` |
| `branch` | object | - | Configuraciones de estrategia de rama |
| `label` | object | - | Configuraciones de estrategia de etiqueta |
| `milestone` | object | - | Configuraciones de estrategia de hito |
| `auto_create` | object | - | Creación automática del siguiente artefacto de lanzamiento |

### Opciones de `release_strategy.branch`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `pattern` | string | `"release/{{version}}"` | Patrón de nomenclatura de rama |
| `base_branch` | string | `"main"` | Rama para comparar |
| `delete_after_release` | bool | `false` | Eliminar rama después del despliegue en producción |

### Opciones de `release_strategy.label`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `pattern` | string | `"release:{{version}}"` | Patrón de nomenclatura de etiqueta |
| `pending_label` | string | - | Etiqueta para PRs en espera de asignación de lanzamiento |
| `remove_after_release` | bool | `false` | Eliminar etiquetas después del despliegue en producción |

### Opciones de `release_strategy.milestone`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `pattern` | string | `"v{{version}}"` | Patrón de nomenclatura de hito |
| `close_after_release` | bool | `false` | Cerrar hito después del despliegue en producción |

### Opciones de `release_strategy.auto_create`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `enabled` | bool | `false` | Habilitar creación automática al completar la etapa final |
| `next_version` | string | `"patch"` | Incremento de versión: `"patch"`, `"minor"`, `"major"` |
| `create_issue` | bool | `false` | Crear nuevo issue de aprobación para el próximo lanzamiento |
| `comment` | string | - | Comentario a publicar sobre el próximo lanzamiento |

### Opciones de `semver`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `prefix` | string | `"v"` | Prefijo de etiqueta |
| `strategy` | string | `"input"` | Estrategia de versión: `"input"`, `"auto"` |
| `validate` | bool | `false` | Validar formato semver |
| `allow_prerelease` | bool | `false` | Permitir versiones preliminares (por ejemplo, `v1.0.0-beta.1`) |
| `auto` | object | - | Configuraciones de incremento automático basado en etiquetas |

### Opciones de `semver.auto`

| Clave | Tipo | Predeterminado | Descripción |
|-------|------|---------------|-------------|
| `major_labels` | string[] | `[]` | Etiquetas de PR que desencadenan un aumento mayor |
| `minor_labels` | string[] | `[]` | Etiquetas de PR que desencadenan un aumento menor |
| `patch_labels` | string[] | `[]` | Etiquetas de PR que desencadenan un aumento de parche |

---

## Detalles de la Funcionalidad

### Palabras Clave de Aprobación

Los usuarios pueden aprobar o denegar solicitudes comentando en el issue:

**Palabras clave de aprobación:** `approve`, `approved`, `lgtm`, `yes`, `/approve`

**Palabras clave de denegación:** `deny`, `denied`, `reject`, `rejected`, `no`, `/deny`

### Soporte de Equipos

Para usar aprobadores basados en equipos de GitHub, necesitas permisos elevados. El `GITHUB_TOKEN` estándar no puede listar miembros del equipo. Usa un token de aplicación de GitHub:

```yaml
jobs:
  process:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Generar token de aplicación de GitHub
      - uses: actions/create-github-app-token@v2
        id: app-token
        with:
          app-id: ${{ vars.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}

      # Usar el token de la aplicación para verificar membresía de equipo
      - uses: jamengual/enterprise-approval-engine@v1
        with:
          action: process-comment
          issue_number: ${{ github.event.issue.number }}
          token: ${{ steps.app-token.outputs.token }}
```

**Permisos requeridos de la aplicación de GitHub:**

- `Organization > Members: Read` - Para listar miembros del equipo

### Tuberías de Despliegue Progresivo

Rastrea despliegues a través de múltiples entornos con un solo issue de aprobación. A medida que cada etapa es aprobada, el issue se actualiza para mostrar el progreso y avanza automáticamente a la siguiente etapa.

#### Configuración de la Tubería

```yaml
# .github/approvals.yml o configuración externa
version: 1

policies:
  developers:
    approvers: [dev1, dev2, dev3]
    min_approvals: 1

  qa-team:
    approvers: [qa1, qa2]
    min_approvals: 1

  tech-leads:
    approvers: [lead1, lead2]
    min_approvals: 1

  production-approvers:
    approvers: [sre1, sre2, security-lead]
    require_all: true

workflows:
  deploy:
    description: "Desplegar a través de todos los entornos (dev → qa → stage → prod)"
    require:
      - policy: developers  # Aprobación inicial para iniciar la tubería
    pipeline:
      track_prs: true       # Incluir PRs en el cuerpo del issue
      track_commits: true   # Incluir commits en el cuerpo del issue
      stages:
        - name: dev
          environment: development
          policy: developers
          on_approved: "✅ ¡Despliegue en **DEV** aprobado! Procediendo a QA..."
        - name: qa
          environment: qa
          policy: qa-team
          on_approved: "✅ ¡Despliegue en **QA** aprobado! Procediendo a STAGING..."
        - name: stage
          environment: staging
          policy: tech-leads
          on_approved: "✅ ¡Despliegue en **STAGING** aprobado! Listo para PRODUCCIÓN..."
        - name: prod
          environment: production
          policy: production-approvers
          on_approved: "🚀 ¡Despliegue en **PRODUCCIÓN** completo!"
          create_tag: true   # Crear etiqueta cuando PROD es aprobado
          is_final: true     # Cerrar issue después de esta etapa
    on_approved:
      close_issue: true
      comment: |
        🎉 **¡Despliegue Completo!**

        La versión `{{version}}` ha sido desplegada en todos los entornos.
```

#### Ejemplo de Flujo de Trabajo de la Tubería

```yaml
# .github/workflows/request-pipeline.yml
name: Solicitar Despliegue de Tubería

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Versión a desplegar'
        required: true
        type: string

permissions:
  contents: write
  issues: write
  pull-requests: read  # Requerido para el seguimiento de PRs

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Necesario para la comparación de commits/PRs

      - uses: jamengual/enterprise-approval-engine@v1
        id: approval
        with:
          action: request
          workflow: deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Output Results
        run: |
          echo "## Despliegue de Tubería Iniciado" >> $GITHUB_STEP_SUMMARY
          echo "- **Issue:** #${{ steps.approval.outputs.issue_number }}" >> $GITHUB_STEP_SUMMARY
          echo "- **URL:** ${{ steps.approval.outputs.issue_url }}" >> $GITHUB_STEP_SUMMARY
```

#### Cómo Funciona

1. **Creación de Issue**: Cuando se activa, crea un solo issue mostrando todas las etapas con un diagrama visual de Mermaid y una tabla de progreso:

```markdown
## 🚀 Tubería de Despliegue: v1.2.0

### Flujo de la Tubería

​```mermaid
flowchart LR
    DEV(⏳ DEV)
    QA(⬜ QA)
    STAGE(⬜ STAGE)
    PROD(⬜ PROD)
    DEV --> QA --> STAGE --> PROD

    classDef completed fill:#28a745,stroke:#1e7e34,color:#fff
    classDef current fill:#ffc107,stroke:#d39e00,color:#000
    classDef pending fill:#6c757d,stroke:#545b62,color:#fff
    class DEV current
    class QA,STAGE,PROD pending
​```

### Progreso del Despliegue

| Etapa | Estado | Aprobador | Hora |
|-------|--------|-----------|------|
| DEV | ⏳ En espera | - | - |
| QA | ⬜ Pendiente | - | - |
| STAGE | ⬜ Pendiente | - | - |
| PROD | ⬜ Pendiente | - | - |

**Etapa Actual:** DEV
```

El diagrama de Mermaid proporciona una vista rápida con nodos codificados por color:
- 🟢 **Verde** - Etapas completadas
- 🟡 **Amarillo** - Etapa actual en espera de aprobación
- ⚪ **Gris** - Etapas pendientes
- 🔵 **Cian** - Etapas de aprobación automática

Para desactivar el diagrama de Mermaid, establece `show_mermaid_diagram: false` en la configuración de la tubería.

2. **Progresión de Etapas**: Comenta `approve` para avanzar a la siguiente etapa. Tanto el diagrama como la tabla se actualizan automáticamente:

```markdown
| Etapa | Estado | Aprobador | Hora |
|-------|--------|-----------|------|
| DEV | ✅ Desplegado | @developer1 | 9 de diciembre 10:30 |
| QA | ✅ Desplegado | @qa-lead | 9 de diciembre 14:15 |
| STAGE | ⏳ En espera | - | - |
| PROD | ⬜ Pendiente | - | - |

**Etapa Actual:** STAGE
```

3. **Seguimiento de PRs y Commits**: Los gestores de lanzamientos ven exactamente qué se está desplegando:

```markdown
### Pull Requests en este Lanzamiento

| PR | Título | Autor |
|----|--------|-------|
| [#42](https://...) | Añadir autenticación de usuario | @alice |
| [#45](https://...) | Corregir error de procesamiento de pagos | @bob |

### Commits

- [`abc1234`](https://...) feat: añadir soporte OAuth2
- [`def5678`](https://...) fix: manejar pagos nulos
```

4. **Finalización**: Cuando la etapa final es aprobada:
   - Se crea una etiqueta (si `create_tag: true`)
   - Se publica un comentario de finalización
   - El issue se cierra automáticamente

#### Opciones de Etapas de la Tubería

| Opción | Descripción |
|--------|-------------|
| `name` | Nombre de la etapa (mostrado en la tabla) |
| `environment` | Nombre del entorno de GitHub |
| `policy` | Política de aprobación para esta etapa |
| `approvers` | Aprobadores en línea (alternativa a la política) |
| `on_approved` | Mensaje a publicar cuando la etapa es aprobada |
| `create_tag` | Crear una etiqueta git en esta etapa |
| `is_final` | Cerrar el issue después de esta etapa |
| `auto_approve` | Aprobar automáticamente esta etapa sin intervención humana |
| `approval_mode` | Sobrescribir el modo de aprobación del flujo de trabajo para esta etapa |

#### Modos de Aprobación

Elige cómo los aprobadores interactúan con las solicitudes de aprobación:

| Modo | Descripción |
|------|-------------|
| `comments` | (Predeterminado) Los aprobadores comentan `/approve` o `approve` en el issue |
| `sub_issues` | Crea un sub-issue para cada etapa - cerrar para aprobar |
| `hybrid` | Mezcla modos por etapa - usa `approval_mode` en cada etapa |

**Ejemplo de Aprobación con Sub-Issue:**

```yaml
workflows:
  deploy:
    approval_mode: sub_issues
    sub_issue_settings:
      title_template: "⏳ Aprobar: {{stage}} para {{version}}"  # Cambia a ✅ cuando es aprobado
      labels: [approval-stage]
      protection:
        only_assignee_can_close: true   # Previene aprobaciones no autorizadas
        prevent_parent_close: true       # El padre no puede cerrar hasta que todos sean aprobados
    pipeline:
      stages:
        - name: dev
          policy: developers
        - name: prod
          policy: production-approvers
```

Con sub-issues, el issue padre muestra una tabla de sub-issues de aprobación:

```markdown
### 📋 Sub-Issues de Aprobación

| Etapa | Sub-Issue | Estado | Asignados |
|-------|-----------|--------|-----------|
| DEV | #124 | ⏳ En espera | @alice, @bob |
| PROD | #125 | ⏳ En espera | @sre1, @sre2 |
```

**Modo Híbrido (sobrescribir por etapa):**

```yaml
workflows:
  deploy:
    approval_mode: comments  # Predeterminado para este flujo de trabajo
    pipeline:
      stages:
        - name: dev
          policy: developers
          # Usa comentarios (predeterminado del flujo de trabajo)
        - name: prod
          policy: production-approvers
          approval_mode: sub_issues  # Sobrescribir solo para producción
```

#### UX Mejorada de Comentarios

La acción incluye una UX mejorada basada en comentarios para la aprobación:

- **Reacciones de Emoji**: Reacciones automáticas en comentarios de aprobación
  - 👍 Aprobado
  - 👎 Denegado
  - 👀 Visto (procesando)

- **Sección de Acciones Rápidas**: El cuerpo del issue incluye una tabla de referencia de comandos:

```markdown
### ⚡ Acciones Rápidas

| Acción | Comando | Descripción |
|--------|---------|-------------|
| ✅ Aprobar | `/approve` | Aprobar la etapa **DEV** |
| ❌ Denegar | `/deny [razón]` | Denegar con razón opcional |
| 📊 Estado | `/status` | Mostrar estado actual de aprobación |
```

**Configurar a través de `comment_settings`:**

```yaml
workflows:
  deploy:
    comment_settings:
      react_to_comments: true     # Añadir reacciones de emoji (predeterminado: true)
      show_quick_actions: true    # Mostrar sección de Acciones Rápidas (predeterminado: true)
```

#### Aprobación Automática para Entornos Inferiores

Usa `auto_approve: true` en etapas de la tubería que deben ser aprobadas automáticamente sin intervención humana. Esto es ideal para entornos inferiores como `dev` o `integration` donde deseas acelerar la tubería mientras mantienes puertas de aprobación para producción.

**Ejemplo con aprobación automática:**

```yaml
workflows:
  deploy:
    description: "Desplegar a través de entornos"
    pipeline:
      stages:
        - name: dev
          environment: development
          auto_approve: true              # Aprobado automáticamente
          on_approved: "🤖 DEV desplegado automáticamente"
        - name: integration
          environment: integration
          auto_approve: true              # Aprobado automáticamente
          on_approved: "🤖 INTEGRATION desplegado automáticamente"
        - name: staging
          environment: staging
          policy: qa-team                 # Requiere aprobación manual
          on_approved: "✅ STAGING aprobado"
        - name: production
          environment: production
          policy: production-approvers    # Requiere aprobación manual
          create_tag: true
          is_final: true
```

**Cómo funciona:**

1. Cuando se crea un issue de tubería, todas las etapas iniciales con `auto_approve: true` se completan automáticamente
2. Cuando una etapa es aprobada manualmente, cualquier etapa consecutiva con `auto_approve: true` que siga también se completa automáticamente
3. Las etapas aprobadas automáticamente se muestran con el indicador 🤖 en la tabla de la tubería
4. El aprobador se registra como `[auto]` en el historial de la etapa

**Casos de uso:**

- **Entornos de desarrollo**: Desplegar inmediatamente sin esperar aprobación
- **Pruebas de integración**: Permitir que la tubería CI/CD progrese automáticamente a través de entornos de prueba
- **Despliegues canarios**: Aprobar automáticamente la etapa canaria, requerir aprobación para el despliegue completo

#### Opciones de Configuración de la Tubería

| Opción | Predeterminado | Descripción |
|--------|---------------|-------------|
| `track_prs` | `false` | Incluir PRs fusionados en el cuerpo del issue |
| `track_commits` | `false` | Incluir commits desde la última etiqueta |
| `compare_from_tag` | - | Patrón de etiqueta personalizado para comparar desde |
| `show_mermaid_diagram` | `true` | Mostrar diagrama de flujo visual de Mermaid de las etapas de la tubería |

**Nota:** El seguimiento de PRs requiere permiso `pull-requests: read` en tu flujo de trabajo.

### Estrategias de Candidato de Lanzamiento

En entornos empresariales, los PRs fusionados en main no siempre son candidatos de lanzamiento inmediatos. El motor de aprobación admite tres estrategias para seleccionar qué PRs pertenecen a un lanzamiento:

#### Tipos de Estrategia

| Estrategia | Descripción | Caso de Uso |
|------------|-------------|-------------|
| `tag` | PRs entre dos etiquetas git (predeterminado) | Lanzamientos simples, desarrollo basado en trunk |
| `branch` | PRs fusionados en una rama de lanzamiento | GitFlow, ramas de lanzamiento |
| `label` | PRs con una etiqueta de lanzamiento específica | Selección flexible, lanzamientos por lotes |
| `milestone` | PRs asignados a un hito de GitHub | Lanzamientos alineados con la hoja de ruta |

#### Configuración

```yaml
# .github/approvals.yml
workflows:
  deploy:
    description: "Tubería de despliegue de producción"
    pipeline:
      track_prs: true
      track_commits: true

      # Configurar estrategia de selección de lanzamiento
      release_strategy:
        type: milestone  # o: tag, branch, label

        # Configuraciones de estrategia de hito
        milestone:
          pattern: "v{{version}}"        # por ejemplo, "v1.2.0"
          close_after_release: true       # Cerrar hito al completar producción

        # Crear automáticamente el siguiente artefacto de lanzamiento al completar
        auto_create:
          enabled: true
          next_version: patch             # o: minor, major
          create_issue: true              # Crear nuevo issue de aprobación

      stages:
        - name: dev
          policy: developers
        - name: prod
          policy: production-approvers
          is_final: true
```

#### Estrategia de Rama

Usa ramas de lanzamiento para desarrollo estilo GitFlow:

```yaml
release_strategy:
  type: branch
  branch:
    pattern: "release/{{version}}"  # Crea release/v1.2.0
    base_branch: main               # Comparar contra main
    delete_after_release: true      # Limpiar después del despliegue en producción

  auto_create:
    enabled: true
    next_version: minor
```

**Cómo funciona:**
1. Crear una rama de lanzamiento: `release/v1.2.0`
2. Los PRs fusionados en la rama son candidatos de lanzamiento
3. Solicitar aprobación para esa versión
4. El issue de aprobación muestra todos los PRs en la rama de lanzamiento
5. Después de producción, la rama se elimina (opcional) y se crea la siguiente rama

#### Estrategia de Etiqueta

Usa etiquetas para una selección flexible de PRs:

```yaml
release_strategy:
  type: label
  label:
    pattern: "release:{{version}}"      # por ejemplo, "release:v1.2.0"
    pending_label: "pending-release"    # Aplicado a PRs fusionados en espera de asignación de lanzamiento
    remove_after_release: true          # Eliminar etiqueta después del despliegue en producción

  auto_create:
    enabled: true
    next_version: patch
```

**Cómo funciona:**
1. Los PRs fusionados en main obtienen la etiqueta `pending-release`
2. El gestor de lanzamientos aplica `release:v1.2.0` a los PRs seleccionados
3. Solicitar aprobación para v1.2.0
4. El issue de aprobación muestra solo los PRs con esa etiqueta
5. Después de producción, las etiquetas se eliminan y se crea la siguiente etiqueta de lanzamiento

#### Estrategia de Hito

Usa hitos para lanzamientos alineados con la hoja de ruta:

```yaml
release_strategy:
  type: milestone
  milestone:
    pattern: "Release {{version}}"       # por ejemplo, "Release 1.2.0"
    close_after_release: true            # Cerrar hito al completar

  auto_create:
    enabled: true
    next_version: minor
    create_issue: true                   # Crear automáticamente el siguiente issue de aprobación
```

**Cómo funciona:**
1. Crear hito: "Release 1.2.0"
2. Asignar PRs al hito durante el desarrollo
3. Solicitar aprobación para v1.2.0
4. El issue de aprobación muestra todos los PRs en el hito
5. Después de producción, el hito se cierra y se crea el siguiente hito

#### Creación Automática al Completar

Cuando la etapa final (producción) es aprobada, prepara automáticamente el siguiente lanzamiento:

```yaml
auto_create:
  enabled: true
  next_version: patch      # Calcular siguiente: patch, minor, o major
  create_issue: true       # Crear nuevo issue de aprobación inmediatamente
  comment: |               # Mensaje personalizado (opcional)
    🚀 **Próximo lanzamiento preparado:** {{version}}
```

Esto crea:
- **Estrategia de rama:** Nueva rama de lanzamiento desde main
- **Estrategia de etiqueta:** Nueva etiqueta de lanzamiento
- **Estrategia de hito:** Nuevo hito

#### Opciones de Limpieza

Cada estrategia tiene acciones de limpieza opcionales que se ejecutan cuando la etapa final (producción) es aprobada. **Todas las opciones de limpieza predeterminan a `false`** - la limpieza es optativa:

| Estrategia | Opción de Limpieza | Descripción |
|------------|--------------------|-------------|
| Rama | `delete_after_release` | Eliminar la rama de lanzamiento |
| Etiqueta | `remove_after_release` | Eliminar etiquetas de lanzamiento de los PRs |
| Hito | `close_after_release` | Cerrar el hito |

```yaml
release_strategy:
  type: branch
  branch:
    pattern: "release/{{version}}"
    delete_after_release: false   # Mantener rama para referencia (predeterminado)

  type: milestone
  milestone:
    pattern: "v{{version}}"
    close_after_release: true     # Cerrar hito cuando se complete
```

#### Despliegues de Hotfix

Para correcciones de emergencia que necesitan omitir los flujos de trabajo de lanzamiento normales, crea un flujo de trabajo separado:

```yaml
# .github/approvals.yml
workflows:
  # Lanzamientos estándar - tubería completa con seguimiento de hitos
  deploy:
    description: "Tubería de lanzamiento estándar (dev → qa → stage → prod)"
    pipeline:
      release_strategy:
        type: milestone
        milestone:
          pattern: "v{{version}}"
          close_after_release: true
        auto_create:
          enabled: true
          next_version: minor
      stages:
        - name: dev
          policy: developers
        - name: qa
          policy: qa-team
        - name: stage
          policy: tech-leads
        - name: prod
          policy: production-approvers
          is_final: true

  # Hotfixes - omitir etapas, directo a producción
  hotfix:
    description: "Hotfix de emergencia - directo a producción"
    pipeline:
      release_strategy:
        type: tag              # Basado en etiquetas simples, no se necesita limpieza
        # No auto_create - los hotfixes son únicos
      stages:
        - name: prod
          policy: production-approvers
          create_tag: true
          is_final: true
    on_approved:
      close_issue: true
      comment: "🚨 Hotfix {{version}} desplegado en producción"
```

**Activar hotfix vs lanzamiento regular:**

```bash
# Lanzamiento regular - pasa por todas las etapas
gh workflow run request-approval.yml -f workflow_name=deploy -f version=v1.3.0

# Hotfix - va directamente a producción
gh workflow run request-approval.yml -f workflow_name=hotfix -f version=v1.2.1
```

**Patrones de Hotfix:**

| Escenario | Estrategia | Limpieza | Creación Automática |
|-----------|------------|----------|---------------------|
| Corrección de emergencia | `tag` | Ninguna | Deshabilitada |
| Lanzamiento de parche | `milestone` | `close_after_release: false` | Deshabilitada |
| Múltiples hotfixes | `branch` | `delete_after_release: false` | Deshabilitada |

#### Beneficios de la Estrategia de Lanzamiento

| Estrategia | Pros | Contras |
|------------|------|--------|
| **Tag** | Simple, sin flujo de trabajo adicional | Todos los PRs fusionados incluidos |
| **Branch** | Alcance de lanzamiento claro, aislamiento | Sobrecarga de gestión de ramas |
| **Label** | Selección flexible, fácil de cambiar | Requiere etiquetado manual |
| **Milestone** | Visibilidad de la hoja de ruta, integración de planificación | Requiere disciplina de hitos |

**Recomendación:**

- Usa **tag** para proyectos simples con despliegue continuo
- Usa **branch** para entornos regulados que necesitan aislamiento de lanzamiento
- Usa **label** para lanzamientos por lotes con alcance flexible
- Usa **milestone** para desarrollo impulsado por la hoja de ruta con planificación de lanzamientos clara

### Integración con Jira

Extrae automáticamente issues de Jira de commits y nombres de ramas. La acción admite dos modos:

#### Modo Solo Enlaces (No se requiere autenticación)

Solo proporciona `jira_base_url` para extraer claves de issues y mostrarlas como enlaces clicables:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: production-deploy
    version: v1.2.0
    token: ${{ secrets.GITHUB_TOKEN }}
    jira_base_url: https://yourcompany.atlassian.net  # ¡Eso es todo!
```

Esto extrae claves de issues (por ejemplo, `PROJ-123`) de mensajes de commit y nombres de ramas, mostrándolas como enlaces en el issue de aprobación:

```markdown
### Issues de Jira
- [PROJ-123](https://yourcompany.atlassian.net/browse/PROJ-123)
- [PROJ-456](https://yourcompany.atlassian.net/browse/PROJ-456)
```

#### Modo Completo (Con Acceso a la API)

Añade credenciales para también obtener detalles de issues y actualizar Fix Versions:

Espero que esta traducción te sea útil. Si necesitas más ayuda, no dudes en preguntar.

Aquí tienes la traducción del texto proporcionado al español natural:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: production-deploy
    version: v1.2.0
    token: ${{ secrets.GITHUB_TOKEN }}
    # Configuración de Jira
    jira_base_url: https://yourcompany.atlassian.net
    jira_user_email: ${{ secrets.JIRA_EMAIL }}
    jira_api_token: ${{ secrets.JIRA_API_TOKEN }}
    jira_update_fix_version: 'true'
```

Esto muestra información detallada sobre los problemas:

```markdown
### Problemas de Jira en esta versión

| Clave | Resumen | Tipo | Estado |
|-------|---------|------|--------|
| [PROJ-123](https://...) | Corregir error de inicio de sesión | Error | Hecho |
| [PROJ-456](https://...) | Añadir modo oscuro | Funcionalidad | En progreso |
```

**Comparación de modos:**

| Modo | Autenticación requerida | Características |
|------|-------------------------|-----------------|
| Solo enlaces | No | Claves de problemas como enlaces clicables |
| Completo | Sí | Enlaces + resumen, estado, emojis de tipo, actualizaciones de versión de corrección |

**Salidas de Jira:**

```yaml
- name: Usar salidas de Jira
  run: |
    echo "Problemas: ${{ steps.approval.outputs.jira_issues }}"
    # Salida: PROJ-123,PROJ-456

    echo "Detalles: ${{ steps.approval.outputs.jira_issues_json }}"
    # Salida: [{"key":"PROJ-123","summary":"Corregir error de inicio de sesión",...}]
```

### Seguimiento de Despliegue

Crea despliegues de GitHub para visibilidad en el panel de despliegues de GitHub. Esto funciona independientemente de la clave `environment:` en el YAML del flujo de trabajo.

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  id: approval
  with:
    action: request
    workflow: production-deploy
    version: v1.2.0
    token: ${{ secrets.GITHUB_TOKEN }}
    # Seguimiento de despliegue
    create_deployment: 'true'
    deployment_environment: production
    deployment_environment_url: https://myapp.example.com

- name: Actualizar estado de despliegue
  if: steps.approval.outputs.status == 'approved'
  run: |
    # Usa el deployment_id para actualizar el estado después del despliegue real
    echo "ID de despliegue: ${{ steps.approval.outputs.deployment_id }}"
```

**Dónde aparecen los despliegues:**

- Pestaña **Despliegues** del repositorio
- Insignias de estado del entorno en la página del repositorio
- Integración de GitHub para Jira (si está configurada)
- API de GitHub para herramientas CI/CD

**Nota:** Esto crea despliegues a través de la API de Despliegues de GitHub, que es independiente de las Reglas de Protección de Entorno nativas de GitHub. Puedes usar ambas juntas o por separado.

### Repositorio de Configuración Externa

Almacena configuraciones de aprobación en un repositorio compartido para una gestión centralizada de políticas:

```yaml
- uses: jamengual/enterprise-approval-engine@v1
  with:
    action: request
    workflow: production-deploy
    token: ${{ secrets.GITHUB_TOKEN }}
    config_repo: myorg/.github  # Repositorio de configuración compartido
```

**Orden de resolución de configuración:**

1. `{repo-name}_approvals.yml` en el repositorio externo (por ejemplo, `myapp_approvals.yml`)
2. `approvals.yml` en el repositorio externo (predeterminado compartido)
3. `.github/approvals.yml` en el repositorio actual (respaldo local)

**Ejemplo de estructura organizativa:**

```text
myorg/.github/
├── myapp_approvals.yml      # Configuración específica de la aplicación
├── backend_approvals.yml    # Configuración de repositorios de backend
└── approvals.yml            # Predeterminado para todos los repositorios
```

### Aprobaciones Bloqueantes

Para flujos de trabajo que necesitan esperar aprobación antes de continuar:

```yaml
name: Desplegar con Aprobación Bloqueante

on:
  workflow_dispatch:
    inputs:
      version:
        required: true
        type: string

jobs:
  request-approval:
    runs-on: ubuntu-latest
    outputs:
      issue_number: ${{ steps.request.outputs.issue_number }}
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        id: request
        with:
          action: request
          workflow: production-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

  wait-for-approval:
    needs: request-approval
    runs-on: ubuntu-latest
    outputs:
      status: ${{ steps.check.outputs.status }}
      tag: ${{ steps.check.outputs.tag }}
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        id: check
        with:
          action: check
          issue_number: ${{ needs.request-approval.outputs.issue_number }}
          wait: 'true'           # Esperar hasta aprobado/denegado
          timeout: '4h'          # Tiempo máximo de espera
          token: ${{ secrets.GITHUB_TOKEN }}

  deploy:
    needs: [request-approval, wait-for-approval]
    if: needs.wait-for-approval.outputs.status == 'approved'
    runs-on: ubuntu-latest
    steps:
      - name: Desplegar
        run: |
          echo "Desplegando ${{ needs.wait-for-approval.outputs.tag }}"
```

**Nota:** Los flujos de trabajo bloqueantes mantienen el runner activo, lo que consume minutos de GitHub Actions. Para escenarios sensibles al costo, usa el enfoque basado en eventos (flujo de trabajo `process-comment` separado).

### Eliminación de Etiquetas al Cerrar Problemas

Opcionalmente elimina etiquetas cuando los problemas de aprobación se cierran manualmente:

```yaml
workflows:
  dev-deploy:
    on_closed:
      delete_tag: true   # Eliminar etiqueta cuando se cierra el problema
      comment: "Cancelado. Etiqueta {{tag}} eliminada."

  production-deploy:
    on_closed:
      delete_tag: false  # NUNCA eliminar etiquetas de producción
```

**Manejar eventos de cierre:**

```yaml
# .github/workflows/handle-close.yml
name: Manejar Cierre de Problemas

on:
  issues:
    types: [closed]

jobs:
  handle:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jamengual/enterprise-approval-engine@v1
        id: close
        with:
          action: close-issue
          issue_number: ${{ github.event.issue.number }}
          issue_action: ${{ github.event.action }}
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Reportar
        run: |
          echo "Estado: ${{ steps.close.outputs.status }}"
          echo "Etiqueta eliminada: ${{ steps.close.outputs.tag_deleted }}"
```

---

## Ejemplos Completos

### Flujo de Trabajo de Solicitud Completo

```yaml
name: Solicitar Despliegue en Producción

on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Versión a desplegar'
        required: true
        type: string
      environment:
        description: 'Entorno objetivo'
        required: true
        type: choice
        options: [staging, production]

permissions:
  contents: write
  issues: write
  deployments: write

jobs:
  request:
    runs-on: ubuntu-latest
    outputs:
      issue_number: ${{ steps.approval.outputs.issue_number }}
      issue_url: ${{ steps.approval.outputs.issue_url }}
      deployment_id: ${{ steps.approval.outputs.deployment_id }}
      jira_issues: ${{ steps.approval.outputs.jira_issues }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Necesario para comparación de commits

      - uses: jamengual/enterprise-approval-engine@v1
        id: approval
        with:
          action: request
          workflow: ${{ inputs.environment }}-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}
          # Integración con Jira
          jira_base_url: https://mycompany.atlassian.net
          jira_user_email: ${{ secrets.JIRA_EMAIL }}
          jira_api_token: ${{ secrets.JIRA_API_TOKEN }}
          # Seguimiento de despliegue
          create_deployment: 'true'
          deployment_environment: ${{ inputs.environment }}
          deployment_environment_url: https://${{ inputs.environment }}.myapp.com

      - name: Resumen
        run: |
          echo "## Solicitud de Aprobación Creada" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "- **Problema:** #${{ steps.approval.outputs.issue_number }}" >> $GITHUB_STEP_SUMMARY
          echo "- **URL:** ${{ steps.approval.outputs.issue_url }}" >> $GITHUB_STEP_SUMMARY
          echo "- **Problemas de Jira:** ${{ steps.approval.outputs.jira_issues }}" >> $GITHUB_STEP_SUMMARY
          echo "- **Commits:** ${{ steps.approval.outputs.commits_count }}" >> $GITHUB_STEP_SUMMARY
```

### Procesar Comentarios con Soporte de Equipo

```yaml
name: Manejar Comentarios de Aprobación

on:
  issue_comment:
    types: [created]

permissions:
  contents: write
  issues: write

jobs:
  process:
    if: |
      github.event.issue.pull_request == null &&
      contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Generar token de aplicación de GitHub para comprobaciones de membresía de equipo
      - uses: actions/create-github-app-token@v2
        id: app-token
        with:
          app-id: ${{ vars.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}

      - uses: jamengual/enterprise-approval-engine@v1
        id: process
        with:
          action: process-comment
          issue_number: ${{ github.event.issue.number }}
          token: ${{ steps.app-token.outputs.token }}
          # Integración con Jira para actualizar la versión de corrección al aprobar
          jira_base_url: https://mycompany.atlassian.net
          jira_user_email: ${{ secrets.JIRA_EMAIL }}
          jira_api_token: ${{ secrets.JIRA_API_TOKEN }}

      - name: Desencadenar Despliegue
        if: steps.process.outputs.status == 'approved'
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.actions.createWorkflowDispatch({
              owner: context.repo.owner,
              repo: context.repo.repo,
              workflow_id: 'deploy.yml',
              ref: 'main',
              inputs: { version: '${{ steps.process.outputs.tag }}' }
            });
```

### Promoción Multi-Entorno

```yaml
# .github/approvals.yml
version: 1

policies:
  dev-team:
    approvers: [dev1, dev2, dev3]
    min_approvals: 1

  qa-team:
    approvers: [qa1, qa2]
    min_approvals: 1

  prod-team:
    approvers: [team:sre, tech-lead]
    min_approvals: 2

workflows:
  dev-deploy:
    require:
      - policy: dev-team
    on_approved:
      tagging:
        enabled: true
        auto_increment: patch
        env_prefix: "dev-"
      close_issue: true

  staging-deploy:
    require:
      - policy: qa-team
    on_approved:
      tagging:
        enabled: true
        auto_increment: minor
        env_prefix: "staging-"
      close_issue: true

  production-deploy:
    require:
      - policy: prod-team
    on_approved:
      create_tag: true
      close_issue: true
    on_closed:
      delete_tag: false  # Nunca eliminar etiquetas de producción
```

### Usar Salidas en Trabajos Posteriores

```yaml
name: Desplegar con Aprobación

on:
  workflow_dispatch:
    inputs:
      version:
        required: true

jobs:
  approval:
    runs-on: ubuntu-latest
    outputs:
      status: ${{ steps.check.outputs.status }}
      tag: ${{ steps.check.outputs.tag }}
      approvers: ${{ steps.check.outputs.approvers }}
      jira_issues: ${{ steps.request.outputs.jira_issues }}
    steps:
      - uses: actions/checkout@v4

      - uses: jamengual/enterprise-approval-engine@v1
        id: request
        with:
          action: request
          workflow: production-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}
          jira_base_url: https://mycompany.atlassian.net

      - uses: jamengual/enterprise-approval-engine@v1
        id: check
        with:
          action: check
          issue_number: ${{ steps.request.outputs.issue_number }}
          wait: 'true'
          timeout: '2h'
          token: ${{ secrets.GITHUB_TOKEN }}

  deploy:
    needs: approval
    if: needs.approval.outputs.status == 'approved'
    runs-on: ubuntu-latest
    environment: production
    steps:
      - name: Desplegar
        run: |
          echo "Desplegando ${{ needs.approval.outputs.tag }}"
          echo "Aprobado por: ${{ needs.approval.outputs.approvers }}"
          echo "Problemas de Jira: ${{ needs.approval.outputs.jira_issues }}"

  notify:
    needs: [approval, deploy]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - name: Notificar a Slack
        run: |
          if [ "${{ needs.approval.outputs.status }}" == "approved" ]; then
            echo "¡Despliegue de ${{ needs.approval.outputs.tag }} completado!"
          else
            echo "El despliegue fue ${{ needs.approval.outputs.status }}"
          fi
```

---

## Validación de Esquema

Valida tu configuración usando el esquema JSON:

```yaml
# .github/approvals.yml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jamengual/enterprise-approval-engine/main/schema.json

version: 1

policies:
  # ... tu configuración
```

O valida en CI:

```yaml
- name: Validar Configuración
  run: |
    npm install -g ajv-cli
    ajv validate -s schema.json -d .github/approvals.yml
```

---

## Servidor Empresarial de GitHub

La acción es totalmente compatible con el Servidor Empresarial de GitHub. Detecta automáticamente los entornos GHES usando las variables de entorno `GITHUB_SERVER_URL` y `GITHUB_API_URL`.

No se requiere configuración adicional: la acción usará automáticamente los puntos finales de API correctos.

**Limitación de Tasa:**

La acción incluye reintentos automáticos con retroceso exponencial para errores de limitación de tasa. Configuración:

- Retraso inicial: 1 segundo
- Retraso máximo: 60 segundos
- Máximos reintentos: 5
- Jitter: Se añade aleatoriamente 0-500ms para prevenir la estampida

---

## Licencia

Licencia MIT