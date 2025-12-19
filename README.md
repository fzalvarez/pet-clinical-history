## Estado actual del proyecto (MVP)

### ✅ Objetivo del servicio
Este repositorio implementa un **Historial Clínico Veterinario** enfocado únicamente en:
- Mascotas (perfil básico)
- Dueños (usuario owner) + **acceso delegado** (delegación)
- Timeline de eventos (consultas médicas, antipulgas, desparasitación, baños, notas, etc.)

**Importante:** este servicio **NO incluye IAM propio** ni planes. La autenticación se integrará luego vía **Odin-IAM**. Los planes/capabilities se resolverán fuera (p.ej. plans-features).

---

## Estado técnico

### ✅ Compila y levanta API
- `go build .\cmd\api` ✅
- `go run .\cmd\api` ✅

### ✅ Router y middleware
- Framework HTTP: **chi**
- Endpoint de salud:
  - `GET /health` → `ok`
- Middleware de auth:
  - Soporta **modo dev** sin verifier: `X-Debug-User-ID`
  - Cuando exista verifier real (Odin), el middleware podrá poblar claims desde `Authorization: Bearer <token>`

### ✅ Persistencia (temporal)
- Repositorios **in-memory** (`internal/adapters/storage/memory`)

---

## Funcionalidades implementadas (MVP)

### 1) Mascotas (Pets)
- **Crear mascota**
  - `POST /pets/`
  - Requiere usuario (claims) → en dev: `X-Debug-User-ID`
  - Owner de la mascota = `claims.UserID`
- **Listar mascotas por owner**
  - `GET /pets/`
  - Requiere usuario (claims)

**Persistencia actual:** repositorios **in-memory** (`internal/adapters/storage/memory`).

---

### 2) Timeline de eventos (Events)
Implementado como módulo separado (no mezclado con pets).

- **Crear evento para una mascota**
  - `POST /pets/{petID}/events/`
  - Requiere usuario (claims)
  - Permisos:
    - Owner: permitido
    - Delegado: requiere grant activo con scope `events:create`
  - `occurred_at` se recibe en RFC3339
  - `recorded_at` se setea automáticamente

- **Listar eventos de una mascota**
  - `GET /pets/{petID}/events/`
  - Requiere usuario (claims)
  - Permisos:
    - Owner: permitido
    - Delegado: requiere grant activo con scope `events:read`

- **Filtros básicos** (según handler actualizado)
  - `limit`
  - `types` (CSV)
  - `from` / `to` (RFC3339)
  - `q` (búsqueda simple en title/notes)

**Persistencia actual:** repositorio **in-memory** con orden por `OccurredAt` descendente.

---

### 3) Delegación (AccessGrants)
Permite que el owner comparta acceso a una mascota con otro usuario (delegado), con scopes.

**Estados soportados:**
- `invited`
- `active`
- `revoked`

**Scopes soportados (base):**
- `events:read`
- `events:create`
- (otros scopes ya definidos en el dominio, pero aún no aplicados en endpoints)

**Endpoints:**
- **Invitar delegado** (owner)
  - `POST /pets/{petID}/grants/`
- **Listar grants por mascota** (owner)
  - `GET /pets/{petID}/grants/`
- **Listar mis grants** (delegado)
  - `GET /me/grants/`
  - Opcional: `?status=invited,active` (CSV)
- **Aceptar invitación** (delegado)
  - `POST /grants/{grantID}/accept`
- **Revocar grant** (owner)
  - `POST /grants/{grantID}/revoke`

---

## Modo dev (sin Odin-IAM)
Mientras `AuthVerifier` sea `nil`, se puede probar con:
- Header: `X-Debug-User-ID: user-123`

---

## Flujos de uso (modo dev)

> Nota: en estos flujos, `X-Debug-User-ID` simula al usuario autenticado (owner / delegado).

### Flujo 1 — Owner crea mascota y registra un evento (sin delegación)
1) **Crear mascota (owner)**
   - `POST /pets/` con `X-Debug-User-ID: owner-1`
2) **Crear evento para esa mascota (owner)**
   - `POST /pets/{petID}/events/` con `X-Debug-User-ID: owner-1`
3) **Listar eventos**
   - `GET /pets/{petID}/events/` con `X-Debug-User-ID: owner-1`

---

### Flujo 2 — Delegación completa: invitar → aceptar → leer/crear events
1) **Owner crea mascota**
   - `POST /pets/` con `X-Debug-User-ID: owner-1`
2) **Owner invita delegado (con scopes)**
   - `POST /pets/{petID}/grants/` con `X-Debug-User-ID: owner-1`
   - Ejemplo scopes: `["events:read","events:create"]`
3) **Delegado lista sus grants para obtener `grantID`**
   - `GET /me/grants/` con `X-Debug-User-ID: delegate-1`
4) **Delegado acepta la invitación**
   - `POST /grants/{grantID}/accept` con `X-Debug-User-ID: delegate-1`
5) **Delegado lista eventos** (requiere `events:read`)
   - `GET /pets/{petID}/events/` con `X-Debug-User-ID: delegate-1`
6) **Delegado crea un evento** (requiere `events:create`)
   - `POST /pets/{petID}/events/` con `X-Debug-User-ID: delegate-1`

---

### Flujo 3 — Delegación “solo lectura”: delegado puede ver pero no crear
1) Owner invita con scopes mínimos:
   - `POST /pets/{petID}/grants/` con body scopes: `["events:read"]`
2) Delegado acepta:
   - `POST /grants/{grantID}/accept`
3) Delegado:
   - ✅ Puede `GET /pets/{petID}/events/`
   - ❌ No puede `POST /pets/{petID}/events/` (debe responder 403)

---

### Flujo 4 — Revocar acceso: el delegado pierde acceso inmediatamente
1) Owner revoca grant:
   - `POST /grants/{grantID}/revoke` con `X-Debug-User-ID: owner-1`
2) Delegado intenta nuevamente:
   - ❌ `GET /pets/{petID}/events/` → 403
   - ❌ `POST /pets/{petID}/events/` → 403

---

## Ejemplos rápidos (curl)

### Health
```bash
curl http://localhost:8080/health
