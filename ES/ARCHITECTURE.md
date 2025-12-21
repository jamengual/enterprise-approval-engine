# Acción de Aprobaciones IssueOps - Diseño de Arquitectura

## Resumen

Una Acción de GitHub para flujos de trabajo de aprobación basados en políticas con:
- **Requisitos de aprobación por grupo** (X de N aprobadores por grupo)
- **Lógica OR entre grupos** (cualquier grupo puede satisfacer la aprobación)
- **Lógica AND dentro de los grupos** (mínimo de aprobadores requeridos)
- **Creación de etiquetas Semver** tras la aprobación
- **Flujo de trabajo basado en issues** para transparencia y seguimiento de auditoría

## Conceptos Básicos

### Modelo de Aprobación

```
Solicitud de Aprobación
├── Grupo A (OR)         ← Cualquier grupo que satisfaga su requisito = aprobado
│   ├── Usuario 1 (AND)  ← Dentro del grupo: se necesitan X de N usuarios/equipos
│   ├── Usuario 2
│   └── Equipo X
├── Grupo B (OR)
│   ├── Equipo Y
│   └── Equipo Z
└── Grupo C (OR)
    └── Usuario 3
```

**Lógica:**
- **Entre grupos**: OR (cualquier grupo que cumpla su umbral aprueba la solicitud)
- **Dentro de los grupos**: AND con umbral (se necesitan X de N miembros para aprobar)

### Escenarios de Ejemplo

1. **Simple**: Cualquiera 2 de [alice, bob, charlie] aprueban → aprobado
2. **Basado en equipos**: Cualquiera 1 del `@org/platform-team` aprueba → aprobado
3. **OR multi-grupo**: (2 del equipo de plataforma) O (1 del equipo de seguridad) → aprobado
4. **Escalación**: (2 del equipo de desarrollo) O (1 de los gerentes) → aprobado

---

## Formato de Configuración

### Ubicación

```
.github/
├── approvals.yml           # Archivo de configuración principal
└── ISSUE_TEMPLATE/
    └── approval-request.yml  # Opcional: plantilla de issue personalizada
```

### Esquema: `.github/approvals.yml`

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/owner/issueops-approvals/main/schema.json
version: 1

# Valores predeterminados globales (opcional)
defaults:
  timeout: 72h                    # Tiempo máximo de espera para aprobaciones
  allow_self_approval: false      # El solicitante no puede aprobar su propia solicitud
  issue_labels:                   # Etiquetas añadidas a los issues de aprobación
    - "approval-required"
    - "issueops"

# Políticas de aprobación - definir grupos de aprobación reutilizables
policies:
  # Política simple: cualquiera 2 de estos usuarios
  dev-review:
    approvers:
      - alice
      - bob
      - charlie
    min_approvals: 2

  # Política basada en equipos
  platform-team:
    approvers:
      - team:platform-engineers    # Prefijo 'team:' para equipos de la organización
    min_approvals: 1

  # Usuarios y equipos mixtos
  security-review:
    approvers:
      - team:security
      - security-lead              # Usuario individual
    min_approvals: 1

# Los flujos de trabajo definen cuándo y cómo se solicitan las aprobaciones
workflows:
  # Aprobación de despliegue en producción
  production-deploy:
    description: "El despliegue en producción requiere aprobación"

    # Condiciones de activación (coinciden con las entradas del flujo de trabajo)
    trigger:
      environment: production

    # Requisito de aprobación: políticas combinadas con lógica OR
    # Cualquiera de estos grupos de políticas satisfechos = aprobado
    require:
      # Opción 1: 2 ingenieros de plataforma aprueban
      - policy: platform-team
        min_approvals: 2          # Sobrescribir el valor predeterminado de la política

      # Opción 2: 1 miembro del equipo de seguridad aprueba
      - policy: security-review

      # Opción 3: Tanto alice como bob aprueban (grupo en línea)
      - approvers: [alice, bob]
        min_approvals: 2          # TODOS deben aprobar

    # Configuración del issue
    issue:
      title: "Aprobación Requerida: Despliegue en Producción - {{version}}"
      labels:
        - "production"
        - "deploy"
      assignees_from_policy: true   # Asignar usuarios de las políticas de aprobación

    # Acciones tras la aprobación
    on_approved:
      create_tag: true              # Crear etiqueta semver
      close_issue: true
      comment: "¡Aprobado! Etiqueta {{version}} creada."

    on_denied:
      close_issue: true
      comment: "Despliegue denegado por {{denier}}."

  # Despliegue en staging - requisitos más simples
  staging-deploy:
    description: "Despliegue en staging"
    trigger:
      environment: staging
    require:
      - policy: dev-review
        min_approvals: 1
    on_approved:
      create_tag: true
      tag_prefix: "staging-"       # Crea: staging-v1.2.3

# Configuración de Semver
semver:
  # Formato de etiqueta
  prefix: "v"                       # v1.2.3

  # Cómo determinar la próxima versión
  strategy: input                   # 'input' = desde la entrada del flujo de trabajo
                                    # 'auto' = auto-incremento basado en etiquetas
                                    # 'conventional' = desde commits convencionales

  # Para la estrategia 'auto'
  auto:
    major_labels: ["breaking", "major"]
    minor_labels: ["feature", "enhancement"]
    patch_labels: ["fix", "bugfix", "patch"]

  # Validación
  validate: true                    # Asegurar formato semver válido
  allow_prerelease: true            # Permitir v1.2.3-beta.1
```

### Configuración Mínima

Para casos de uso simples:

```yaml
version: 1

policies:
  approvers:
    approvers: [alice, bob, charlie]
    min_approvals: 2

workflows:
  default:
    require:
      - policy: approvers
```

---

## Interfaz de Acción

### Entradas

```yaml
- uses: owner/issueops-approvals@v1
  with:
    # Requerido
    action: request|check|approve|deny    # Qué operación realizar

    # Para la acción 'request'
    workflow: production-deploy           # Qué configuración de flujo de trabajo usar
    version: "1.2.3"                      # Versión Semver (si se crea etiqueta)

    # Para la acción 'check'
    issue_number: 123                     # Issue para verificar estado

    # Para las acciones 'approve'/'deny' (usadas por el flujo de trabajo de comentarios)
    # Estas se activan típicamente por eventos de issue_comment

    # Autenticación (requerida para verificaciones de membresía de equipo)
    token: ${{ secrets.GITHUB_TOKEN }}    # Operaciones básicas
    app_id: ${{ vars.APP_ID }}            # Para membresía de equipo (opcional)
    app_private_key: ${{ secrets.KEY }}   # Para membresía de equipo (opcional)

    # Sobrescrituras opcionales
    config_path: .github/approvals.yml    # Ubicación de configuración personalizada
    timeout: 48h                          # Sobrescribir tiempo de espera
```

### Salidas

```yaml
outputs:
  status: approved|pending|denied|timeout
  issue_number: "123"
  issue_url: "https://github.com/..."
  approvers: "alice,bob"                  # Quién aprobó
  tag: "v1.2.3"                           # Etiqueta creada (si aplica)
  approval_groups_satisfied: "platform-team"
```

---

## Patrones de Flujo de Trabajo

### Patrón 1: Solicitar y Esperar (Bloqueante)

```yaml
name: Deploy with Approval

on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [staging, production]
      version:
        type: string
        required: true

jobs:
  request-approval:
    runs-on: ubuntu-latest
    outputs:
      issue_number: ${{ steps.request.outputs.issue_number }}
    steps:
      - uses: owner/issueops-approvals@v1
        id: request
        with:
          action: request
          workflow: ${{ inputs.environment }}-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}

  wait-for-approval:
    needs: request-approval
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        id: approval
        with:
          action: check
          issue_number: ${{ needs.request-approval.outputs.issue_number }}
          token: ${{ secrets.GITHUB_TOKEN }}
          wait: true                      # Esperar hasta resolver
          timeout: 24h

      - if: steps.approval.outputs.status != 'approved'
        run: exit 1

  deploy:
    needs: wait-for-approval
    runs-on: ubuntu-latest
    steps:
      - run: echo "Deploying..."
```

### Patrón 2: Basado en Eventos (No Bloqueante)

```yaml
# Flujo de trabajo 1: Solicitar aprobación (no bloqueante)
name: Request Deployment Approval

on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [staging, production]
      version:
        type: string

jobs:
  request:
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        with:
          action: request
          workflow: ${{ inputs.environment }}-deploy
          version: ${{ inputs.version }}
          token: ${{ secrets.GITHUB_TOKEN }}
      # El flujo de trabajo sale - la aprobación se maneja por el flujo de trabajo de comentarios
```

```yaml
# Flujo de trabajo 2: Manejar comentarios de aprobación
name: Process Approval Comments

on:
  issue_comment:
    types: [created]

jobs:
  process:
    if: contains(github.event.issue.labels.*.name, 'approval-required')
    runs-on: ubuntu-latest
    steps:
      - uses: owner/issueops-approvals@v1
        id: process
        with:
          action: process-comment
          token: ${{ secrets.GITHUB_TOKEN }}
          app_id: ${{ vars.APP_ID }}
          app_private_key: ${{ secrets.APP_PRIVATE_KEY }}

      - if: steps.process.outputs.status == 'approved'
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.actions.createWorkflowDispatch({
              owner: context.repo.owner,
              repo: context.repo.repo,
              workflow_id: 'deploy.yml',
              ref: 'main',
              inputs: {
                version: '${{ steps.process.outputs.version }}'
              }
            })
```

---

## Arquitectura de Implementación

### Componentes

```
issueops-approvals/
├── cmd/
│   └── action/
│       └── main.go              # Punto de entrada de la acción
├── internal/
│   ├── config/
│   │   ├── config.go            # Análisis de configuración
│   │   ├── schema.go            # Validación de esquema JSON
│   │   └── types.go             # Tipos de configuración
│   ├── approval/
│   │   ├── engine.go            # Motor de lógica de aprobación
│   │   ├── policy.go            # Evaluación de políticas
│   │   └── status.go            # Seguimiento de estado
│   ├── github/
│   │   ├── client.go            # Cliente API de GitHub
│   │   ├── issues.go            # Operaciones de issues
│   │   ├── teams.go             # Membresía de equipos
│   │   └── tags.go              # Creación de etiquetas
│   ├── semver/
│   │   ├── parse.go             # Análisis de Semver
│   │   ├── validate.go          # Validación
│   │   └── increment.go         # Lógica de auto-incremento
│   └── action/
│       ├── request.go           # Acción de solicitud
│       ├── check.go             # Acción de verificación
│       ├── process.go           # Acción de procesamiento de comentarios
│       └── outputs.go           # Salidas de la acción
├── schema.json                   # Esquema JSON para validación de configuración
├── action.yml                    # Metadatos de la acción
├── Dockerfile                    # Acción Docker
└── README.md
```

### Tipos Básicos

```go
// Config representa la configuración de approvals.yml
type Config struct {
    Version   int                    `yaml:"version"`
    Defaults  Defaults               `yaml:"defaults"`
    Policies  map[string]Policy      `yaml:"policies"`
    Workflows map[string]Workflow    `yaml:"workflows"`
    Semver    SemverConfig           `yaml:"semver"`
}

// Policy define un grupo de aprobación reutilizable
type Policy struct {
    Approvers    []string `yaml:"approvers"`     // Usuarios o "team:name"
    MinApprovals int      `yaml:"min_approvals"` // Conteo requerido
}

// Workflow define un flujo de trabajo de aprobación
type Workflow struct {
    Description string           `yaml:"description"`
    Trigger     map[string]any   `yaml:"trigger"`
    Require     []Requirement    `yaml:"require"`   // OR entre estos
    Issue       IssueConfig      `yaml:"issue"`
    OnApproved  ActionConfig     `yaml:"on_approved"`
    OnDenied    ActionConfig     `yaml:"on_denied"`
}

// Requirement es un camino de aprobación (políticas combinadas con OR)
type Requirement struct {
    Policy       string   `yaml:"policy"`        // Referencia a la política
    Approvers    []string `yaml:"approvers"`     // Aprobadores en línea
    MinApprovals int      `yaml:"min_approvals"` // Sobrescribir o en línea
}

// ApprovalStatus rastrea el estado actual
type ApprovalStatus struct {
    State              string                      // pending|approved|denied
    GroupsStatus       map[string]GroupStatus      // Estado por grupo
    Approvals          []Approval                  // Todas las aprobaciones recibidas
    Denials            []Denial                    // Todas las denegaciones recibidas
    SatisfiedGroup     string                      // Qué grupo fue satisfecho
}

type GroupStatus struct {
    Required  int        // Aprobaciones mínimas necesarias
    Current   int        // Aprobaciones recibidas
    Approvers []string   // Quién aprobó
    Satisfied bool       // ¿Se cumplió el umbral?
}
```

### Lógica del Motor de Aprobación

```go
// CheckApprovalStatus evalúa todos los requisitos (lógica OR)
func (e *Engine) CheckApprovalStatus(req *ApprovalRequest) *ApprovalStatus {
    status := &ApprovalStatus{
        State:        "pending",
        GroupsStatus: make(map[string]GroupStatus),
    }

    // Recopilar todas las aprobaciones de los comentarios de issues
    approvals := e.collectApprovals(req.IssueNumber)
    denials := e.collectDenials(req.IssueNumber)

    // Verificar si existe ALGUNA denegación (configurable)
    if len(denials) > 0 && req.Workflow.FailOnDeny {
        status.State = "denied"
        status.Denials = denials
        return status
    }

    // Evaluar cada grupo de requisitos (lógica OR entre grupos)
    for _, requirement := range req.Workflow.Require {
        groupStatus := e.evaluateRequirement(requirement, approvals)
        status.GroupsStatus[requirement.Name()] = groupStatus

        // Lógica OR: si ALGÚN grupo está satisfecho, la aprobación está completa
        if groupStatus.Satisfied {
            status.State = "approved"
            status.SatisfiedGroup = requirement.Name()
            break
        }
    }

    return status
}

// evaluateRequirement verifica si un solo grupo cumple su umbral
func (e *Engine) evaluateRequirement(req Requirement, approvals []Approval) GroupStatus {
    // Obtener aprobadores elegibles para este requisito
    eligible := e.getEligibleApprovers(req)

    // Contar aprobaciones de usuarios elegibles
    count := 0
    var approvers []string
    for _, approval := range approvals {
        if e.isEligible(approval.User, eligible) {
            count++
            approvers = append(approvers, approval.User)
        }
    }

    required := req.MinApprovals
    if required == 0 {
        required = 1 // Predeterminado a 1
    }

    return GroupStatus{
        Required:  required,
        Current:   count,
        Approvers: approvers,
        Satisfied: count >= required,
    }
}
```

---

## Plantilla de Issue

Los issues de aprobación generados usan este formato:

```markdown
## Solicitud de Aprobación: Despliegue en Producción v1.2.3

**Solicitado por:** @requester
**Entorno:** producción
**Versión:** v1.2.3
**Solicitado el:** 2024-01-15 10:30:00 UTC

---

### Requisitos de Aprobación

Esta solicitud puede ser aprobada por **cualquiera** de los siguientes:

| Grupo | Requerido | Actual | Estado |
|-------|----------|---------|--------|
| Equipo de Plataforma | 2 de 3 | 0 | ⏳ Pendiente |
| Revisión de Seguridad | 1 de 2 | 0 | ⏳ Pendiente |

### Cómo Aprobar

Comenta con uno de: `approve`, `approved`, `lgtm`, `/approve`

### Cómo Denegar

Comenta con uno de: `deny`, `denied`, `/deny`

---

### Registro de Aprobación

<!-- issueops-approvals-state:{"version":"1.2.3","workflow":"production-deploy"} -->
```

---

## Consideraciones de Seguridad

1. **Permisos del Token**
   - GITHUB_TOKEN básico: Lectura/escritura de issues, sin acceso a equipos
   - GitHub App: Añadir `members:read` para verificaciones de membresía de equipo

2. **Prevención de Auto-Aprobación**
   - Configurable por flujo de trabajo
   - Predeterminado: el solicitante no puede aprobar su propia solicitud

3. **Validación**
   - Verificar que el comentarista esté en la lista de aprobadores elegibles
   - Verificar membresía de equipo vía API (requiere token de App)
   - Validar formato semver antes de crear etiquetas

4. **Registro de Auditoría**
   - Todas las aprobaciones/denegaciones registradas como comentarios de issues
   - Tiempos y atribución de usuario preservados
   - La línea de tiempo del issue proporciona un historial completo

---

## Comparación con Acciones Existentes

| Característica | trstringer/manual-approval | Esta Acción |
|----------------|----------------------------|-------------|
| Umbrales por grupo | No (solo total) | Sí |
| Lógica OR entre grupos | No | Sí |
| Basado en archivo de configuración | No (solo entradas) | Sí |
| Políticas reutilizables | No | Sí |
| Creación de etiquetas Semver | No | Sí |
| Plantillas de issues personalizadas | Limitado | Sí |
| Soporte de equipos | Sí (con App) | Sí (con App) |
| Patrón basado en eventos | Polling | Ambos |

---

## Fases de Implementación

### Fase 1: MVP Básico
- [ ] Análisis y validación de configuración
- [ ] Motor de aprobación básico (un solo grupo, umbral mínimo)
- [ ] Creación de issues con tabla de estado
- [ ] Procesamiento de comentarios (aprobar/denegar)
- [ ] Validación de semver y creación de etiquetas

### Fase 2: Soporte Multi-Grupo
- [ ] Lógica OR entre grupos de requisitos
- [ ] Seguimiento de umbrales por grupo
- [ ] Actualizaciones de tabla de estado
- [ ] Detección de grupo satisfecho

### Fase 3: Integración de Equipos
- [ ] Soporte de token de GitHub App
- [ ] Resolución de membresía de equipo
- [ ] Aprobadores mixtos de usuario/equipo

### Fase 4: Funciones Avanzadas
- [ ] Estrategias de auto-incremento de semver
- [ ] Coincidencia de condiciones de activación
- [ ] Plantillas de issues personalizadas
- [ ] Manejo de tiempo de espera
- [ ] Comentarios de recordatorio

### Fase 5: Pulido
- [ ] Esquema JSON para validación de configuración
- [ ] Documentación completa
- [ ] Ejemplos de flujos de trabajo
- [ ] Cobertura de pruebas