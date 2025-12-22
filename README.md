## Estado actual del proyecto (MVP)

### ✅ Objetivo del servicio
Este repositorio implementa un **Historial Clínico Veterinario** enfocado únicamente en:
- Mascotas (perfil básico)
- Dueños (usuario owner) + **acceso delegado** (delegación)
- Timeline de eventos (consultas médicas, antipulgas, desparasitación, baños, notas, etc.)

**Importante:** este servicio **NO incluye IAM propio** ni planes.  
La autenticación se integrará luego vía **Odin-IAM**. Los planes/capabilities se resolverán fuera (p.ej. plans-features).

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

## Regla de autorización (policy)

Para endpoints que operan sobre una mascota (`{petID}`):

- **Owner** (p.OwnerUserID == claims.UserID): ✅ permitido (owner bypass)
- **Delegado**: ✅ permitido solo si existe un grant **active** para ese `petID` y el `claims.UserID`,
  y el grant incluye el **scope requerido**
- Caso contrario: ❌ `403 forbidden`

---

## Funcionalidades implementadas (MVP)

### 1) Mascotas (Pets)

#### Endpoints
- **Crear mascota**
  - `POST /pets/`
  - Requiere usuario (claims) → en dev: `X-Debug-User-ID`
  - Owner de la mascota = `claims.UserID`

- **Listar mascotas del owner**
  - `GET /pets/`
  - Requiere usuario (claims)

- **Ver mascota por ID**
  - `GET /pets/{petID}`
  - Permisos:
    - Owner: permitido
    - Delegado: requiere grant activo con scope `pet:read`

- **Editar perfil de mascota**
  - `PATCH /pets/{petID}`
  - Permisos:
    - Owner: permitido
    - Delegado: requiere grant activo con scope `pet:edit_profile`
  - PATCH real:
    - campo ausente → no se modifica
    - `birth_date: null` → limpia fecha
    - `birth_date: "YYYY-MM-DD"` → setea fecha

- **Listar mascotas compartidas conmigo**
  - `GET /me/pets`
  - Devuelve mascotas donde existe grant `active` con scope `pet:read`

**Persistencia actual:** repositorios **in-memory** (`internal/adapters/storage/memory`).

---

### 2) Timeline de eventos (Events)
Implementado como módulo separado (no mezclado con pets).

#### Endpoints
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

- **Anular evento (void)**
  - `POST /pets/{petID}/events/{eventID}/void`
  - Requiere usuario (claims)
  - Permisos:
    - Owner: permitido
    - Delegado: requiere grant activo con scope `events:void`
  - No borra: marca `status=voided`

#### Filtros (contrato estable)
`GET /pets/{petID}/events/` acepta:

- `limit` (int) → default `50`, max `200`
- `types` (CSV) → ejemplo: `types=MEDICAL_VISIT,BATH`
- `from` (RFC3339) → ejemplo: `from=2025-12-01T00:00:00-05:00`
- `to` (RFC3339)
- `q` (string) → búsqueda simple en `title` + `notes`

**Orden:** resultados por `occurred_at` descendente (más reciente primero).  
**Persistencia actual:** repositorio **in-memory**.

---

### 3) Delegación (AccessGrants)
Permite que el owner comparta acceso a una mascota con otro usuario (delegado), con scopes.

#### Estados soportados
- `invited`
- `active`
- `revoked`

#### Scopes soportados (base)
- `pet:read`
- `pet:edit_profile`
- `events:read`
- `events:create`
- `events:void`

> Nota: en la invitación, si se envían scopes vacíos, el servicio puede aplicar defaults mínimos (según implementación).

#### Endpoints
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

### Flujo 2 — Delegación completa: invitar → aceptar → ver mascota → leer/crear events
1) **Owner crea mascota**
   - `POST /pets/` con `X-Debug-User-ID: owner-1`
2) **Owner invita delegado (con scopes)**
   - `POST /pets/{petID}/grants/` con `X-Debug-User-ID: owner-1`
   - Ejemplo scopes: `["pet:read","events:read","events:create"]`
3) **Delegado lista sus grants para obtener `grantID`**
   - `GET /me/grants/` con `X-Debug-User-ID: delegate-1`
4) **Delegado acepta la invitación**
   - `POST /grants/{grantID}/accept` con `X-Debug-User-ID: delegate-1`
5) **Delegado ve el perfil de la mascota** (requiere `pet:read`)
   - `GET /pets/{petID}` con `X-Debug-User-ID: delegate-1`
6) **Delegado lista eventos** (requiere `events:read`)
   - `GET /pets/{petID}/events/` con `X-Debug-User-ID: delegate-1`
7) **Delegado crea un evento** (requiere `events:create`)
   - `POST /pets/{petID}/events/` con `X-Debug-User-ID: delegate-1`

---

### Flujo 3 — Delegación “solo lectura”: delegado puede ver pero no crear
1) Owner invita con scopes mínimos:
   - `POST /pets/{petID}/grants/` con body scopes: `["pet:read","events:read"]`
2) Delegado acepta:
   - `POST /grants/{grantID}/accept`
3) Delegado:
   - ✅ Puede `GET /pets/{petID}`
   - ✅ Puede `GET /pets/{petID}/events/`
   - ❌ No puede `POST /pets/{petID}/events/` (debe responder 403)

---

### Flujo 4 — Revocar acceso: el delegado pierde acceso inmediatamente
1) Owner revoca grant:
   - `POST /grants/{grantID}/revoke` con `X-Debug-User-ID: owner-1`
2) Delegado intenta nuevamente:
   - ❌ `GET /pets/{petID}` → 403
   - ❌ `GET /pets/{petID}/events/` → 403
   - ❌ `POST /pets/{petID}/events/` → 403

---

## Ejemplos rápidos (curl)

### Crear mascota (owner)
```bash
curl -X POST http://localhost:8080/pets/ ^
  -H "Content-Type: application/json" ^
  -H "X-Debug-User-ID: owner-1" ^
  -d "{\"name\":\"Luna\",\"species\":\"dog\",\"breed\":\"mixed\",\"sex\":\"female\",\"birth_date\":\"2021-04-10\",\"notes\":\"\"}"

### Invitar delegado
```bash
curl -X POST http://localhost:8080/pets/{petID}/grants/ ^
  -H "Content-Type: application/json" ^
  -H "X-Debug-User-ID: owner-1" ^
  -d "{\"grantee_user_id\":\"delegate-1\",\"scopes\":[\"pet:read\",\"events:read\",\"events:create\"]}"

### Delegado lista sus grants
```bash
curl http://localhost:8080/me/grants ^
  -H "X-Debug-User-ID: delegate-1"

### Delegado acepta invitación
```bash
curl -X POST http://localhost:8080/grants/{grantID}/accept ^
  -H "X-Debug-User-ID: delegate-1"

### Crear evento
```bash
curl -X POST http://localhost:8080/pets/{petID}/events/ ^
  -H "Content-Type: application/json" ^
  -H "X-Debug-User-ID: owner-1" ^
  -d "{\"type\":\"BATH\",\"occurred_at\":\"2025-12-21T10:00:00-05:00\",\"title\":\"Baño\",\"notes\":\"Todo ok\"}"

### Listar eventos con filtros
```bash
curl "http://localhost:8080/pets/{petID}/events/?limit=50&types=BATH,MEDICAL_VISIT&from=2025-12-01T00:00:00-05:00&q=ba%C3%B1o" ^
  -H "X-Debug-User-ID: owner-1"

### Void evento
```bash
curl -X POST http://localhost:8080/pets/{petID}/events/{eventID}/void ^
  -H "X-Debug-User-ID: owner-1"

### Health
```bash
curl http://localhost:8080/health
