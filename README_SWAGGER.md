# Swagger / OpenAPI - Instalación y Uso

Este proyecto usa **swaggo/swag** para generar documentación OpenAPI automática a partir de anotaciones en el código.

---

## 1. Instalar swag CLI

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Verifica instalación:

```bash
swag --version
```

---

## 2. Instalar dependencias del proyecto

```bash
go get -u github.com/swaggo/swag
go get -u github.com/swaggo/http-swagger
go mod tidy
```

---

## 3. Generar documentación

Desde la raíz del proyecto:

```bash
swag init -g cmd/api/main.go --output docs
```

Esto generará/actualizará:
- `docs/docs.go`
- `docs/swagger.json`
- `docs/swagger.yaml`

---

## 4. Ejecutar el servidor

```bash
go run cmd/api/main.go
```

---

## 5. Ver la documentación

Abre en tu navegador:

```
http://localhost:8080/swagger/index.html
```

Verás la UI interactiva de Swagger con todos los endpoints documentados.

---

## 6. Modo dev (sin token real)

Para probar endpoints protegidos sin Odin-IAM, usa el header `X-Debug-User-ID`:

1. En Swagger UI, haz clic en **"Authorize"**
2. En el campo `DebugUserID`, ingresa un user ID (ej: `user-123`)
3. Haz clic en **"Authorize"** y luego **"Close"**
4. Ahora todos los requests incluirán `X-Debug-User-ID: user-123`

---

## 7. Regenerar docs después de cambios

Cada vez que modifiques anotaciones Swagger (`// @Summary`, `// @Param`, etc.), vuelve a ejecutar:

```bash
swag init -g cmd/api/main.go --output docs
```

---

## Notas

- Los handlers ya tienen anotaciones completas en:
  - `internal/domain/pets/handler.go`
  - `internal/domain/events/handler.go`
  - `internal/domain/accessgrants/handler.go`
- El endpoint `/health` **no** está documentado (no tiene anotaciones Swagger); solo aparece en el código.
- Para producción, considera mover `@host` a variable de entorno o archivo de configuración.
