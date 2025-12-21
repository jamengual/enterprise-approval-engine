# Investigación del Ecosistema de Aprobación de IssueOps

## Resumen Ejecutivo

Este documento proporciona una investigación exhaustiva sobre el ecosistema de aprobación de IssueOps y GitHub Actions. Tras analizar las herramientas disponibles, la documentación y los comentarios de la comunidad, se identifica una clara oportunidad para una nueva acción que aborde brechas específicas en el ecosistema evitando la duplicación de esfuerzos.

**Hallazgo Clave:** Aunque existen varias acciones de aprobación, presentan limitaciones significativas en cuanto a la flexibilidad de configuración, aprobaciones basadas en políticas, registros de auditoría y características empresariales. Una nueva acción centrada en **configuración estructurada (JSON/YAML), aprobaciones basadas en políticas, registros de auditoría completos y estrategias de aprobación flexibles** llenaría vacíos genuinos.

---

## 1. Qué Existe en el Ecosistema

### 1.1 Acciones Oficiales de IssueOps (issue-ops.github.io)

La referencia oficial de IssueOps enumera estas acciones principales:

| Acción | Propósito | Formato de Configuración |
|--------|-----------|--------------------------|
| **issue-ops/parser** | Convierte respuestas de formularios de issues a JSON | YAML (plantillas de formularios de issues) |
| **issue-ops/validator** | Valida issues contra plantillas + lógica personalizada | YAML + JavaScript (ESM) |
| **issue-ops/labeler** | Operaciones de etiquetado en lote | No especificado |
| **issue-ops/releaser** | Automatiza la creación de lanzamientos | No especificado |
| **issue-ops/semver** | Gestiona el versionado semántico | No especificado |
| **github/command** | Proporciona funcionalidad de comandos de IssueOps | Flujo de trabajo YAML |
| **actions/add-to-project** | Se integra con tableros de proyectos | Flujo de trabajo YAML |
| **actions/create-github-app-token** | Genera tokens de aplicaciones de GitHub | Flujo de trabajo YAML |

**Perspectivas Clave:**
- Estas acciones se centran en el análisis, validación y automatización
- Ninguna maneja específicamente flujos de trabajo de aprobación con lógica de múltiples aprobadores
- Fuerte énfasis en formularios de issues como interfaz de configuración
- No hay acción de aprobación nativa en el kit de herramientas oficial de IssueOps

### 1.2 Acciones de Aprobación de Terceros

#### trstringer/manual-approval (Más Popular)

**Características Principales:**
- Crea un issue de GitHub para solicitar aprobación
- Soporta usuarios individuales y equipos de organizaciones como aprobadores
- Palabras clave configurables para aprobación/denegación
- Umbrales mínimos de aprobación
- Prevención de autoaprobación
- Título y cuerpo del issue personalizados
- Creación de issues entre repositorios

**Formato de Configuración:**
- Entradas de flujo de trabajo YAML
- Palabras clave de aprobación basadas en texto (insensibles a mayúsculas)
- Sin configuración de políticas estructuradas

**Manejo de Múltiples Aprobadores:**
- Todos los aprobadores asignados al issue
- Requiere que TODOS los aprobadores respondan por defecto
- `minimum-approvals` permite aprobación parcial
- Cualquier denegación falla el flujo de trabajo (configurable)
- Sondea comentarios de issues para palabras clave de aprobación

**Soporte de Equipos de Organización:**
- Requiere token de aplicación de GitHub (no el estándar GITHUB_TOKEN)
- La aplicación necesita permiso de "Miembros de la Organización: lectura"
- Los tokens de la aplicación expiran después de 1 hora (límite estricto en la duración de la aprobación)
- No se pueden asignar issues a equipos (solo a individuos)
- Máximo 10 asignados por issue

**Limitaciones:**
- **Restricciones de Tiempo:**
  - Tiempo de espera de trabajo de 6 horas
  - Tiempo de espera de flujo de trabajo de 35 días
  - Expiración de token de 1 hora para aprobaciones de equipo
- **Impacto en Costos:** Los trabajos pausados consumen ranuras de trabajos concurrentes e incurren en costos de cómputo
- **Límites de Tamaño de Archivo:** Contenido del cuerpo del issue limitado a ~10KB, archivos a ~125KB
- **Plataforma:** Solo ejecutores Linux (sin soporte para Windows)
- **Sin Motor de Políticas:** Solo coincidencia simple de palabras clave
- **Registro de Auditoría Limitado:** Solo línea de tiempo del issue

**Ejemplo de Configuración:**
```yaml
- uses: trstringer/manual-approval@v1
  with:
    secret: ${{ github.TOKEN }}
    approvers: user1,user2,team1
    minimum-approvals: 2
    issue-title: "Se requiere aprobación para despliegue"
    exclude-workflow-initiator-as-approver: true
```

#### joshjohanning/approveops (ApproveOps)

**Características Principales:**
- Validación de aprobación basada en equipos
- Coincidencia de comando único (coincidencia exacta, se ignoran espacios en blanco)
- Comando de aprobación personalizable
- Publicación opcional de comentarios de éxito
- Acción nativa de Node.js 20+

**Formato de Configuración:**
- Entradas de flujo de trabajo YAML
- Cadena de comando única (por defecto: `/approve`)
- Solo referencia de nombre de equipo

**Manejo de Múltiples Aprobadores:**
- CUALQUIER miembro del equipo puede aprobar (una sola aprobación)
- Sin soporte para conteos mínimos de aprobación
- Sin soporte para múltiples equipos
- Membresía de equipo validada vía API

**Limitaciones:**
- **Modelo de Aprobador Único:** Solo se necesita una aprobación, no es un verdadero multi-aprobador
- **Equipo Único:** No se pueden especificar múltiples equipos o listas mixtas de usuarios/equipos
- **Sin Flujo de Trabajo de Denegación:** Solo soporta aprobación, sin ruta de rechazo
- **Sin Configuración de Políticas:** Lógica de aprobación fija
- **Requiere Equipo de GitHub:** Debe tener equipo en la misma organización

**Ejemplo de Configuración:**
```yaml
- uses: joshjohanning/approveops@v3
  with:
    token: ${{ secrets.GITHUB_TOKEN }}
    approve-command: '/approve'
    team-name: 'deployment-approvers'
```

#### kharkevich/issue-ops-approval (Aprobaciones de IssueOps de GitHub)

**Características Principales:**
- Aprobación/declinación basada en comentarios
- Modo de lista (usuarios individuales) o modo de equipo
- Umbrales de aprobación configurables
- Palabras clave personalizadas para aprobación/declinación
- Devuelve salida tri-estatal (aprobado/declinado/indefinido)

**Formato de Configuración:**
- Entradas de flujo de trabajo YAML
- Listas de aprobadores delimitadas por comas
- Cadenas de palabras clave personalizadas

**Manejo de Múltiples Aprobadores:**
- Soporta umbrales mínimos de aprobación
- Rastrea aprobaciones a través de múltiples aprobadores
- Puede usar equipos de organización en modo de equipo
- Declinación opcional (fail-on-decline: false)

**Limitaciones:**
- **Documentación Mínima:** 1 estrella, mantenimiento mínimo
- **Sin Características Avanzadas:** Solo conteo básico de aprobaciones
- **Modo de Equipo Requiere Token de Aplicación:** Al igual que otros, necesita permisos elevados
- **Sin Motor de Políticas:** Lógica de conteo simple
- **Registro de Auditoría Limitado:** Seguimiento básico de comentarios

**Ejemplo de Configuración:**
```yaml
- uses: kharkevich/issue-ops-approval@v1
  with:
    repo-token: ${{ secrets.GITHUB_TOKEN }}
    mode: list
    approvers: user1,user2,user3
    minimum-approvals: 2
    fail-on-decline: true
```

#### Otras Acciones Notables

**ekeel/approval-action:**
- Usa issues del repositorio para aprobaciones
- Configuración de tiempo de espera
- Se ejecuta en Ubuntu, macOS, Windows
- Tercero, no certificado por GitHub

**akefirad/manual-approval-action:**
- Pausa para aprobación manual
- Bueno para despliegues de Terraform (plan antes de aplicar)
- Funcionalidad básica

**toppulous/create-manual-approval-issue:**
- Crea o encuentra issues para aprobación
- Etiquetas únicas por etapa (por ejemplo, "aprobación-dev")
- Más enfocado en la creación de issues que en la lógica de aprobación

### 1.3 Soluciones Nativas de GitHub

#### Entornos (Función Oficial)

**Características Principales:**
- Flujo de trabajo de aprobación basado en UI
- Revisores requeridos (hasta 6 usuarios/equipos)
- Solo 1 revisor necesita aprobar
- Temporizadores de espera
- Restricciones de ramas de despliegue
- Secretos con alcance de entorno
- Reglas de protección de despliegue personalizadas

**Formato de Configuración:**
- Configuración de repositorio (basada en UI)
- Referenciado en flujo de trabajo YAML mediante la clave `environment:`
- Sin configuración basada en código

**Manejo de Múltiples Aprobadores:**
- Hasta 6 revisores
- Solo se requiere UNA aprobación (no se puede requerir múltiples)
- Aprobación a nivel de trabajo (no a nivel de flujo de trabajo)
- Cada trabajo que referencia el entorno requiere aprobación separada

**Limitaciones:**
- **Restricciones de Nivel de Precios:**
  - Gratis/Pro/Equipo: Solo repos públicos obtienen revisores requeridos
  - Repos privados: Requiere GitHub Enterprise
- **Sin Multi-Aprobación:** No se puede requerir N de M aprobadores
- **Solo a Nivel de Trabajo:** Cada trabajo necesita aprobación separada
- **Sin Motor de Políticas:** Solo lista básica de revisores
- **Configuración Basada en UI:** No se puede controlar la versión de la configuración del entorno

---

## 2. Características Clave en Todas las Herramientas

### Capacidades Comunes

1. **Mecanismos de Aprobación:**
   - Aprobaciones basadas en comentarios (palabras clave en comentarios de issues/PR)
   - Soporte para usuarios y equipos de organización
   - Palabras clave de aprobación configurables
   - Palabras clave de denegación/rechazo

2. **Patrones de Configuración:**
   - Entradas de flujo de trabajo YAML (todas las acciones de terceros)
   - Listas de usuarios/equipos delimitadas por comas
   - Coincidencia de palabras clave basada en texto
   - Banderas booleanas (excluir-iniciador, fallar-en-denegación)

3. **Integración:**
   - Pasos de flujo de trabajo de GitHub Actions
   - Monitoreo de comentarios de issues/PR
   - Sondeos de verificaciones de estado
   - Interacción con la API de GitHub

4. **Autenticación:**
   - GITHUB_TOKEN estándar para uso básico
   - Tokens de aplicaciones de GitHub para soporte de equipos
   - Permisos elevados para consultas de membresía de equipos

### Características Distintivas

| Característica | trstringer | ApproveOps | kharkevich | Entornos |
|----------------|-----------|------------|------------|----------|
| Aprobaciones mínimas | Sí | No | Sí | No |
| Soporte de equipos de organización | Sí* | Sí | Sí* | Sí |
| Flujo de trabajo de denegación | Sí | No | Sí | N/A |
| Issues entre repositorios | Sí | No | No | N/A |
| Palabras clave personalizadas | Sí | Sí | Sí | N/A |
| Intervalo de sondeo | Configurable | N/A | N/A | N/A |
| Prevención de autoaprobación | Sí | N/A | No | N/A |
| Estado de salida | Sí | N/A | Tri-estatal | N/A |
| Límites de tiempo | 1hr/6hr/35d | Desconocido | Desconocido | 30 días |
| Repos privados gratuitos | Sí | Sí | Sí | No |

*Requiere token de aplicación de GitHub con permisos de organización

---

## 3. Patrones Comunes de Configuración

### Patrón de Entrada (basado en YAML)

Todas las acciones de terceros siguen patrones de entrada YAML similares:

```yaml
- uses: vendor/action@version
  with:
    # Autenticación
    token/secret: ${{ secrets.TOKEN }}

    # Aprobadores
    approvers: "user1,user2,team1"
    mode: list|team  # Algunas acciones

    # Umbrales
    minimum-approvals: 2

    # Comportamiento
    fail-on-denial: true
    exclude-workflow-initiator-as-approver: true

    # Personalización
    issue-title: "Título Personalizado"
    approve-command: "/approve"
    additional-approved-words: "lgtm,ship-it"
```

### Aprobación Basada en Palabras Clave

Todas las acciones utilizan coincidencia de palabras clave basada en texto:
- **Aprobación:** "approve", "approved", "lgtm", "yes", emojis
- **Denegación:** "deny", "denied", "no"
- Insensible a mayúsculas
- Puntuación opcional
- Recorte de espacios en blanco

### Patrón de Soporte de Equipos

El soporte de equipos de organización requiere permisos elevados:

```yaml
- uses: actions/create-github-app-token@v2
  id: app-token
  with:
    app-id: ${{ vars.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}

- uses: approval-action@v1
  with:
    token: ${{ steps.app-token.outputs.token }}
    approvers: org/team-name
```

### Consumo de Salida

Las acciones proporcionan salidas para lógica condicional:

```yaml
- id: approval
  uses: approval-action@v1

- if: steps.approval.outputs.approval-status == 'approved'
  run: echo "¡Aprobado!"
```

---

## 4. Brechas que una Nueva Acción Podría Llenar

### Brechas Críticas

#### 4.1 Configuración de Aprobación Basada en Políticas

**Estado Actual:**
- Todas las acciones utilizan coincidencia simple de palabras clave
- Sin soporte para políticas de aprobación complejas
- Sin reglas de aprobación condicionales
- Sin versionado de políticas

**Oportunidad:**
- Configuración de políticas basada en JSON/YAML
- Reglas condicionales (basadas en tiempo, ruta, cambios)
- Plantillas de políticas y herencia
- Archivos de políticas versionados en el repositorio
- Integración con patrones de issue-ops/validator

**Ejemplo de Capacidad Faltante:**
```yaml
# Esto no existe en las acciones actuales
approval-policy:
  rules:
    - name: production-deployment
      conditions:
        - environment: production
        - files_changed: "src/**"
      approvers:
        minimum: 2
        required_teams: [platform-team, security-team]
        prevent_self_approval: true
    - name: database-migration
      conditions:
        - files_changed: "migrations/**"
      approvers:
        minimum: 3
        required_roles: [dba, senior-engineer]
```

#### 4.2 Formato de Configuración Estructurada

**Estado Actual:**
- Solo entradas de flujo de trabajo YAML
- Cadenas delimitadas por comas para aprobadores
- Sin validación de esquema
- Sin reutilización de configuración

**Oportunidad:**
- Archivos de configuración JSON/YAML (como el patrón de issue-ops/validator)
- Validación de esquema
- Plantillas de configuración
- Configuración multi-entorno
- Herencia de configuración

**Ejemplo:**
```yaml
# .github/approvals/config.yml
version: 1.0
defaults:
  minimum-approvals: 1
  timeout: 24h

environments:
  staging:
    approvers: [dev-team]
    minimum-approvals: 1
  production:
    approvers: [platform-team, security-team]
    minimum-approvals: 2
    business-hours-only: true
```

#### 4.3 Registro de Auditoría Completo

**Estado Actual:**
- Solo línea de tiempo del issue
- Sin registros de auditoría estructurados
- Sin metadatos de aprobación
- Sin informes de cumplimiento

**Oportunidad:**
- Registros de auditoría estructurados (JSON)
- Metadatos de aprobación (marca de tiempo, aprobador, razón, contexto)
- Informes de cumplimiento
- Integración con sistemas de auditoría externos
- Historial de aprobaciones a través de ejecuciones de flujo de trabajo

#### 4.4 Estrategias Avanzadas de Múltiples Aprobadores

**Estado Actual:**
- Conteo simple (N de M aprobadores)
- Todo o umbral mínimo
- Sin aprobaciones basadas en roles
- Sin aprobadores condicionales

**Oportunidad:**
- Requisitos de aprobación basados en roles
- Listas de aprobadores condicionales (basadas en cambios)
- Aprobaciones ponderadas (algunos aprobadores cuentan más)
- Flujos de trabajo de escalación
- Encadenamiento de aprobaciones (A luego B luego C)

**Ejemplo:**
```json
{
  "approval-strategy": {
    "type": "weighted",
    "threshold": 10,
    "approvers": [
      { "user": "senior-engineer", "weight": 5 },
      { "user": "mid-engineer", "weight": 3 },
      { "team": "juniors", "weight": 1 }
    ]
  }
}
```

#### 4.5 Mejor Gestión del Tiempo

**Estado Actual:**
- Tiempos de espera estrictos (1hr para tokens, 6hr para trabajos)
- Los trabajos pausados consumen recursos
- Sin soporte para horas laborales
- Sin manejo de zonas horarias

**Oportunidad:**
- Aplicación de horas laborales
- Tiempos de espera conscientes de zonas horarias
- Auto-escalación al tiempo de espera
- Períodos de gracia y recordatorios
- Pausa/reanudación sin consumir recursos (mediante flujo de trabajo separado)

#### 4.6 Integración con Formularios de Issues

**Estado Actual:**
- Las acciones crean issues genéricos
- Sin integración con plantillas de formularios de issues
- Construcción manual del cuerpo del issue
- Sin validación de campos

**Oportunidad:**
- Generar issues de aprobación a partir de plantillas de formularios de issues
- Prellenar campos desde el contexto del flujo de trabajo
- Validar respuestas de aprobación usando issue-ops/validator
- Recolección de datos de aprobación estructurada

#### 4.7 Razones y Contexto de Aprobación

**Estado Actual:**
- Palabras clave simples de aprobación/denegación
- Sin contexto de aprobación
- Sin razón requerida
- Sin resumen de cambios

**Oportunidad:**
- Razones de aprobación requeridas
- Resumen del impacto del cambio
- Campos de evaluación de riesgos
- Seguimiento de justificación de aprobación
- Integración con datos de diferencias de PR

#### 4.8 Soporte para Flujos de Trabajo Multi-Etapa

**Estado Actual:**
- Cada etapa requiere aprobación separada
- Sin aprobación a nivel de flujo de trabajo
- Sin dependencias de etapa
- Sin reutilización de aprobación

**Oportunidad:**
- Aprobación a nivel de flujo de trabajo (aprobar una vez, desplegar muchas)
- Dependencias de aprobación de etapa
- Aprobaciones condicionales de etapa
- Alcance de aprobación (etapa única vs. flujo de trabajo completo)

**Ejemplo:**
```yaml
approval-stages:
  - name: initial-approval
    scope: workflow
    approvers: [team-lead]
  - name: production-approval
    scope: job
    depends-on: initial-approval
    approvers: [platform-team]
    conditions:
      - environment: production
```

#### 4.9 Controles de Seguridad Mejorados

**Estado Actual:**
- Prevención básica de autoaprobación
- Sin restricciones de IP
- Sin aplicación de MFA
- Sin delegación de aprobación

**Oportunidad:**
- Lista blanca de IP para aprobaciones
- Requisito de MFA para aprobaciones sensibles
- Cadenas de delegación de aprobación
- Aprobaciones de emergencia de ruptura de vidrio
- Integración de auditoría de seguridad

#### 4.10 Mejor Experiencia del Desarrollador

**Estado Actual:**
- Basado en sondeos (desperdicio)
- Sin visibilidad del estado de aprobación
- Sin recordatorios de aprobación
- Sin mejoras de soporte móvil

**Oportunidad:**
- Basado en webhooks (impulsado por eventos)
- Panel de estado de aprobación
- Sistema de recordatorio de aprobación
- Integración con Slack/Teams/Email
- Plantillas de issues enriquecidas con orientación

---

## 5. Si Crear una Nueva Acción Sería Duplicación vs. Necesidad Genuina

### Preocupaciones de Duplicación

El ecosistema ya tiene:
- Flujos de trabajo de aprobación básicos (trstringer/manual-approval es maduro)
- Opciones de aprobación basadas en equipos (múltiples opciones)
- Patrón de aprobación basado en issues (bien establecido)
- Soporte de nivel gratuito (evitando el requisito de GitHub Enterprise)

Crear otra acción que **solo** haga aprobación básica basada en palabras clave con umbrales mínimos sería pura duplicación.

### Áreas de Necesidad Genuina

Una nueva acción llenaría necesidades genuinas si se enfoca en:

#### 5.1 Enfoque de Configuración Primero

**Por Qué Es Necesario:**
- Las acciones actuales están impulsadas por entradas de flujo de trabajo (no reutilizables)
- Sin forma de controlar la versión de las políticas de aprobación
- Sin validación de esquema
- Se alinea con los patrones del ecosistema de issue-ops (parser/validator usan archivos de configuración)

**Valor Agregado:**
- Políticas de aprobación a nivel de repositorio en `.github/approvals/`
- Validación de esquema JSON
- Plantillas de políticas y herencia
- Más fácil de auditar y revisar cambios en políticas de aprobación

#### 5.2 Motor de Políticas

**Por Qué Es Necesario:**
- Las acciones actuales tienen lógica de aprobación fija
- Sin aprobaciones condicionales
- Sin reglas de negocio complejas
- Las necesidades empresariales requieren flexibilidad

**Valor Agregado:**
- Requisitos de aprobación basados en condiciones
- Aprobaciones basadas en rutas (diferentes aprobadores para diferentes archivos)
- Reglas basadas en tiempo (horas laborales, períodos de apagón)
- Escalación basada en riesgos

**Casos de Uso:**
- Las migraciones de bases de datos requieren aprobación de DBA
- Los cambios de infraestructura requieren equipo de plataforma
- Los archivos de seguridad requieren equipo de seguridad
- Los despliegues de producción requieren múltiples aprobaciones solo durante horas laborales

#### 5.3 Auditoría y Cumplimiento

**Por Qué Es Necesario:**
- La línea de tiempo del issue es insuficiente para el cumplimiento
- Sin registros de auditoría estructurados
- Sin metadatos de aprobación
- El cumplimiento SOC2/ISO requiere registros detallados

**Valor Agregado:**
- Registros de auditoría estructurados en JSON
- Metadatos de aprobación (quién, cuándo, por qué, contexto)
- Informes de cumplimiento
- Integración con sistemas SIEM/auditoría
- Registro de auditoría inmutable

#### 5.4 Profundidad de Integración

**Por Qué Es Necesario:**
- Las acciones actuales son independientes
- Sin integración con otras herramientas de IssueOps
- Construcción manual de issues
- Sin validación de datos de aprobación

**Valor Agregado:**
- Integración profunda con issue-ops/parser y issue-ops/validator
- Generar issues de aprobación a partir de plantillas
- Validar respuestas de aprobación
- Reutilizar patrones del ecosistema de IssueOps

**Flujo de Ejemplo:**
```
1. El flujo de trabajo desencadena la necesidad de aprobación
2. La acción genera un issue de aprobación a partir de la plantilla (integración con formularios de issues)
3. Los aprobadores responden a través de campos de formulario de issues (datos estructurados)
4. El validador asegura que los datos de aprobación estén completos (integración con issue-ops/validator)
5. El parser extrae datos de aprobación (integración con issue-ops/parser)
6. El motor de políticas evalúa la aprobación contra las reglas
7. Se crea un registro de auditoría estructurado
8. El flujo de trabajo continúa o falla
```

#### 5.5 Mejoras en la Experiencia del Desarrollador

**Por Qué Es Necesario:**
- El sondeo es ineficiente
- Sin visibilidad del estado de aprobación
- Sin recordatorios o escalación
- Mala experiencia móvil

**Valor Agregado:**
- Impulsado por eventos (no sondeo)
- Panel de estado de aprobación
- Recordatorios automatizados
- Integración con Slack/Teams
- Contexto de aprobación enriquecido

### Recomendación: Construir una Acción Diferenciada

**Construirla SI te enfocas en:**

1. **Arquitectura Impulsada por Configuración**
   - Archivos de políticas JSON/YAML en `.github/approvals/`
   - Validación de esquema
   - Versionado de políticas
   - Sistema de plantillas

2. **Motor de Políticas Central**
   - Reglas de aprobación condicionales
   - Aprobaciones basadas en rutas
   - Reglas basadas en tiempo
   - Escalación basada en riesgos
   - Estrategias de aprobación flexibles

3. **Integración Profunda con IssueOps**
   - Funciona con issue-ops/parser
   - Funciona con issue-ops/validator
   - Usa plantillas de formularios de issues
   - Sigue patrones del ecosistema de IssueOps

4. **Registro de Auditoría Orientado al Cumplimiento**
   - Registros de auditoría estructurados
   - Metadatos de aprobación
   - Informes de cumplimiento
   - Registro inmutable

5. **Mejor DX**
   - Impulsado por eventos (webhooks)
   - Panel de estado
   - Notificaciones
   - Contexto de aprobación

**Evitar duplicación al NO:**
- Construir solo otra acción de aprobación basada en coincidencia de palabras clave
- Recrear trstringer/manual-approval con pequeños ajustes
- Enfocarse solo en lógica de conteo básica
- Ignorar patrones existentes del ecosistema

### Propuesta de Valor Única

Una nueva acción debería posicionarse como:

> "Flujos de trabajo de aprobación basados en políticas para IssueOps con configuración estructurada, registros de auditoría completos e integración profunda con el ecosistema de issue-ops. Mientras que trstringer/manual-approval proporciona puertas de aprobación básicas, issueops-approvals ofrece políticas de aprobación de nivel empresarial, registros de auditoría listos para cumplimiento y estrategias de aprobación flexibles a través de configuración JSON/YAML."

**Usuarios Objetivo:**
- Equipos empresariales que necesitan cumplimiento (SOC2, ISO)
- Equipos con políticas de aprobación complejas
- Equipos que ya usan el ecosistema de issue-ops
- Equipos que necesitan registros de auditoría e informes
- Equipos que desean políticas de aprobación versionadas

**No Compitiendo Con:**
- Puertas de aprobación simples (usar trstringer/manual-approval)
- Flujos de trabajo de un solo aprobador (usar approveops)
- Entornos nativos de GitHub (si tienes Enterprise)

---

## 6. Matriz de Resumen: Comparación de Acciones

| Característica | trstringer | ApproveOps | kharkevich | Entornos | **Oportunidad de Nueva Acción** |
|----------------|-----------|------------|------------|----------|--------------------------------|
| **Configuración** | Entradas YAML | Entradas YAML | Entradas YAML | Configuración UI | Archivos de políticas JSON/YAML |
| **Motor de Políticas** | No | No | No | No | **SÍ** |
| **Reglas Condicionales** | No | No | No | No | **SÍ** |
| **Registro de Auditoría** | Línea de tiempo del issue | Línea de tiempo del issue | Línea de tiempo del issue | Registros UI | **Registros JSON estructurados** |
| **Validación de Esquema** | No | No | No | No | **SÍ** |
| **Integración con Formularios de Issues** | Manual | Manual | Manual | N/A | **Automática** |
| **Integración con Validator** | No | No | No | N/A | **SÍ** |
| **Integración con Parser** | No | No | No | N/A | **SÍ** |
| **Estrategias de Aprobación** | Básica | Única | Básica | Básica | **Avanzada (ponderada, basada en roles, condicional)** |
| **Gestión del Tiempo** | Tiempo de espera básico | Desconocido | Desconocido | 30 días | **Horas laborales, zonas horarias, escalación** |
| **Características de Cumplimiento** | No | No | No | No | **Registros de auditoría, informes, metadatos** |
| **Soporte Multi-Etapa** | No | No | No | A nivel de trabajo | **A nivel de flujo de trabajo + A nivel de trabajo** |
| **Repos Privados Gratuitos** | Sí | Sí | Sí | No | **SÍ** |
| **Impulsado por Eventos** | No (sondeo) | Desconocido | Desconocido | Sí | **SÍ (webhooks)** |

---

## 7. Estrategia de Implementación Recomendada

Si se construye una nueva acción, seguir esta estrategia de diferenciación:

### Fase 1: Diferenciadores Clave (MVP)
1. Configuración de políticas JSON/YAML en `.github/approvals/`
2. Validación de esquema para políticas
3. Integración con issue-ops/parser para datos de aprobación
4. Registros de auditoría estructurados (salida JSON)
5. Aprobaciones condicionales basadas en rutas

### Fase 2: Características Avanzadas
1. Integración con issue-ops/validator
2. Generación de plantillas de formularios de issues
3. Aplicación de horas laborales
4. Metadatos y contexto de aprobación
5. Informes de cumplimiento

### Fase 3: Características Empresariales
1. Aprobaciones ponderadas
2. Estrategias de aprobación basadas en roles
3. Flujos de trabajo de escalación de aprobación
4. Integración con SIEM
5. UI de panel e informes

### No Construir
- Otro verificador de aprobación basado en sondeos (usar webhooks)
- Otro simple coincidente de palabras clave (trstringer hace esto bien)
- Otro contador básico de aprobaciones mínimas (kharkevich existe)
- Duplicar características sin mejora clara

---

## 8. Conclusión

**El ecosistema de aprobación de IssueOps tiene:**
- Sólidas acciones de aprobación básicas (trstringer/manual-approval es el estándar)
- Buenas opciones de aprobación basadas en equipos (ApproveOps)
- Características empresariales limitadas
- Sin configuración basada en políticas
- Sin registros de auditoría completos
- Sin integración profunda con herramientas del ecosistema de IssueOps

**Una nueva acción sería valiosa SI:**
- Adopta un enfoque de configuración primero (políticas JSON/YAML)
- Construye un motor de políticas para aprobaciones condicionales
- Crea registros de auditoría completos para cumplimiento
- Se integra profundamente con issue-ops/parser y issue-ops/validator
- Se enfoca en necesidades empresariales (cumplimiento, auditoría, políticas complejas)
- Mejora la experiencia del desarrollador (eventos, notificaciones, paneles)

**Una nueva acción sería duplicación SI:**
- Solo hace coincidencia de palabras clave básica
- Solo cuenta aprobaciones hasta un umbral
- No se integra con el ecosistema de IssueOps
- No proporciona características de auditoría/cumplimiento
- No ofrece configuración basada en políticas

**Recomendación:** Construir la acción con un enfoque claro en aprobaciones basadas en políticas, configuración estructurada, registros de auditoría completos e integración profunda con el ecosistema de IssueOps. Esto llenará vacíos genuinos y servirá a usuarios empresariales mientras se evita la duplicación de las excelentes herramientas de aprobación básicas que ya existen.

---

## Fuentes

- [Referencia de Acciones de IssueOps](https://issue-ops.github.io/docs/reference/issueops-actions)
- [Repositorio de GitHub de trstringer/manual-approval](https://github.com/trstringer/manual-approval)
- [Aprobación Manual en un Flujo de Trabajo de GitHub Actions - Thomas Stringer](https://trstringer.com/github-actions-manual-approval/)
- [Aprobación Manual en un Flujo de Trabajo - GitHub Marketplace](https://github.com/marketplace/actions/manual-workflow-approval)
- [IssueOps: Automatiza CI/CD con Issues y Actions de GitHub - Blog de GitHub](https://github.blog/engineering/issueops-automate-ci-cd-and-more-with-github-issues-and-actions/)
- [Aprobaciones de IssueOps de GitHub - GitHub Marketplace](https://github.com/marketplace/actions/github-issueops-approvals)
- [ApproveOps - Aprobaciones en IssueOps - GitHub Marketplace](https://github.com/marketplace/actions/approveops-approvals-in-issueops)
- [Repositorio de GitHub de issue-ops/validator](https://github.com/issue-ops/validator)
- [Validador de IssueOps - GitHub Marketplace](https://github.com/marketplace/actions/issueops-validator)
- [Repositorio de GitHub de issue-ops/parser](https://github.com/issue-ops/parser)
- [Parser de Cuerpo de Issue - GitHub Marketplace](https://github.com/marketplace/actions/issue-body-parser)
- [Repositorio de GitHub de github/command](https://github.com/github/command)
- [Acción de Comando - GitHub Marketplace](https://github.com/marketplace/actions/command-action)
- [Habilitación de Despliegues de Ramas a través de IssueOps - Blog de GitHub](https://github.blog/engineering/engineering-principles/enabling-branch-deployments-through-issueops-with-github-actions/)
- [Revisando Despliegues - Documentación de GitHub](https://docs.github.com/en/actions/managing-workflow-runs/reviewing-deployments)
- [Habilitación de Aprobación Única para Despliegue Multi-Etapa - Medium](https://medium.com/operations-research-bit/enabling-single-approval-for-a-multi-stage-deployment-workflow-in-github-actions-30f898ea74c7)
- [Añadiendo Paso de Aprobación Manual en GitHub Actions - Medium](https://medium.com/@bounouh.fedi/adding-a-manual-approval-step-in-github-actions-for-controlled-deployments-on-free-github-accounts-cf7f05e759cf)
- [Estrategias de Despliegue de GitHub Actions con Entornos - DevToolHub](https://devtoolhub.com/github-actions-deployment-strategies-environments/)
- [Puntos Dolorosos del Flujo de Trabajo de Aprobación de GitHub Actions - Stack Overflow](https://stackoverflow.com/questions/64593034/github-action-manual-approval-process)
- [Opción de Aprobación Única para Todos los Entornos - Discusión de GitHub](https://github.com/orgs/community/discussions/174381)
- [ApproveOps: IssueOps de GitHub con Aprobaciones - josh-ops](https://josh-ops.com/posts/github-approveops/)
- [Flujo de Trabajo de Comentarios - Documentación de IssueOps](https://issue-ops.github.io/docs/setup/comment-workflow)
- [Introducción a IssueOps - Documentación de IssueOps](https://issue-ops.github.io/docs/introduction)
- [Flujos de Trabajo de Aprobación de Despliegue - MOSS](https://moss.sh/reviews/deployment-approval-workflows/)
- [Puertas de Aprobación Condicionales de GitHub Actions - Stack Overflow](https://stackoverflow.com/questions/77458946/github-actions-conditional-approval-gates)
- [Entornos de Despliegue de GitHub y Puertas de Aprobación - Blog de DevOps de Silvana](https://devops.silvanasblog.com/blog/github-action-deployment-gates/)
- [Añadiendo Flujo de Trabajo de Aprobación a GitHub Action - Blog de TO THE NEW](https://www.tothenew.com/blog/adding-approval-workflow-to-your-github-action/)