# Memoria del proyecto — photo-bucket-backend

Resumen vivo del contexto de la API para que Claude retome el trabajo sin tener que releer todo el repo desde cero. Actualizar esta memoria cuando cambie la arquitectura, se resuelvan bugs listados aquí, o se agreguen features nuevas.

## Qué es esto

API REST en Go que gestiona lectura/escritura de archivos multimedia (fotos) usando MinIO como almacenamiento de objetos y PostgreSQL como base de datos relacional (metadatos). Es el backend de un ecosistema más grande:

- **Este repo**: API Go + Gin + GORM + MinIO (en construcción activa).
- **Futuro**: Frontend en Next.js consumiendo esta API.
- **Futuro**: App móvil Android en Flutter consumiendo esta API.

Por eso el diseño de contratos HTTP (JSON shape, códigos de error, auth) importa especialmente: dos clientes distintos (web y mobile) van a depender de la misma API.

## Stack técnico

- Go 1.26, Gin como router/framework HTTP.
- GORM + `gorm.io/driver/postgres` para persistencia.
- MinIO (`minio-go/v7`) para almacenamiento de objetos (buckets, no filesystem local).
- `golang-jwt/jwt/v5` para autenticación por JWT (HS256, claim `sub` = userID).
- `bcrypt` (golang.org/x/crypto) para hashing de passwords.
- `go-playground/validator/v10` para validación de DTOs vía tags `binding`.
- `ulule/limiter/v3` para rate limiting (in-memory).
- `gin-contrib/cors` para CORS.
- `slog` (stdlib) con JSON handler para logging estructurado.
- `godotenv` para cargar `.env` en desarrollo.
- `swaggo/swag` + `swaggo/gin-swagger` + `swaggo/files` para documentación OpenAPI/Swagger (ver sección "Documentación Swagger" abajo).
- Docker: hay `dockerfile` para la API y `minio/docker-compose.yaml` solo para levantar MinIO local (admin/admin12345, puertos 9000/9001). No hay compose que levante API+DB+MinIO juntos todavía.

## Documentación Swagger

La API expone documentación OpenAPI generada con `swaggo/swag` a partir de comentarios `@Summary`/`@Param`/`@Success`/etc. encima de cada handler.

- UI interactiva: `GET /swagger/index.html` (montada en `internal/router/router.go`, registrada antes que el resto de las rutas). Spec crudo en `GET /swagger/doc.json`.
- Anotaciones generales de la API (`@title`, `@BasePath`, `@securityDefinitions.apikey BearerAuth`) están arriba de `func main()` en `main.go`. El esquema de auth documentado es `BearerAuth` (header `Authorization: Bearer <token>`) — cada handler protegido por `AuthRequired` lleva `@Security BearerAuth`.
- Los archivos generados viven en `docs/` (`docs.go`, `swagger.json`, `swagger.yaml`) y **se commitean** al repo (no hay CI que los regenere todavía). `main.go` los importa con blank import (`_ "github.com/laraxpy/photo-bucket-backend/docs"`) para que su `init()` registre el spec.
- **Regenerar después de tocar cualquier anotación o agregar un endpoint nuevo**: `swag init -g main.go --output docs --parseDependency --parseInternal`. Las flags `--parseDependency --parseInternal` son necesarias porque los modelos usan `gorm.DeletedAt` (tipo externo) — sin ellas `swag` falla con `cannot find type definition: gorm.DeletedAt`.
- **Gotcha de nombres de paquete**: `internal/handler/user`, `internal/handler/file` e `internal/handler/folder` se llaman igual que sus paquetes de modelo (`internal/model/user`, `/file`, `/folder`). Para que `swag` resuelva sin ambigüedad a qué paquete se refiere una anotación como `@Success 200 {object} user.User`, cada handler importa el modelo con un alias (`usermodel`, `filemodel`, `foldermodel`) y tiene una línea `var _ usermodel.User` (idem file/folder) solo para forzar esa resolución — no es código funcional, es exclusivamente para desambiguar el parser de swag. No lo elimines pensando que es código muerto.
- `apperror.ErrorResponse`/`apperror.ErrorBody` (`internal/apperror/response.go`) son tipos **solo de documentación**: reflejan el envelope JSON real que arma `middleware.ErrorHandler`, pero el código de errores sigue construyendo la respuesta con `gin.H` en runtime, no con estos structs. Se usan únicamente en las anotaciones `@Failure` para que Swagger muestre el shape correcto.

## Arquitectura (capas)

Patrón: `handler → service → store`, con modelos de dominio separados.

```
internal/
  apperror/      Tipo de error de aplicación (AppError) + traducción de errores de validación
  config/        Carga de variables de entorno (.env) a un struct Config
  database/      Conexión GORM + AutoMigrate
  handler/       Handlers HTTP por dominio (file, user, health, test_ping_pong), cada uno con su router.go
  httpx/         Helper BindAndValidate compartido por todos los handlers
  middleware/    auth (JWT), cors, error_handler, rate_limiter, request_logger
  model/         Structs de dominio + tags GORM (file, user, folder [WIP])
  router/        Punto único de registro de rutas (RegisterRoutes)
  server/        http.Server + graceful shutdown
  service/       Lógica de negocio (interfaces + implementación), inyecta store + minio client
  storage/       Conexión al cliente MinIO
  store/         Acceso a datos vía GORM, un store por agregado, expuesto como interfaz
```

Convención al agregar un dominio nuevo (ej. `folder`):
1. `internal/model/<dominio>/<dominio>.go` — struct GORM con tags correctos (ver bugs conocidos abajo, folder.go está roto).
2. `internal/store/<dominio>_store.go` — interfaz `XStore` + struct `gormXStore` + `NewXStore(db)`.
3. `internal/service/<dominio>_service.go` — interfaz `XService` + struct + `NewXService(...)`, retorna `*apperror.AppError` en errores.
4. `internal/handler/<dominio>/<dominio>_handler.go` + `router.go` — handler recibe el `XService` por interfaz, expone `RegisterRoutes(r, handler, ...)`.
5. Registrar el nuevo router dentro de `internal/router/router.go` (`RegisterRoutes`).
6. Wire manual en `main.go` (no hay contenedor DI): store → service → handler, en ese orden.
7. Agregar el modelo al `AutoMigrate` en `internal/database/database.go`.

No hay inyección de dependencias automática ni wire/fx — todo el cableado es manual en `main.go`. Todos los stores/services se exponen como **interfaces**, no como structs concretos — patrón a preservar (facilita mocks para tests, que hoy no existen).

## Manejo de errores (convención estricta)

- Nunca usar `c.JSON` directo para errores: siempre `c.Error(apperror.XXX(...))` y dejar que `middleware.ErrorHandler()` lo serialice.
- `apperror.AppError` tiene `Code`, `Message`, `HTTPStatus`, `Err` (interno, no se expone), `Fields` (errores de validación por campo).
- Constructores: `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Validation`, `Internal`, `NoMethod`, `TooManyRequest`.
- Validación de DTOs: usar `httpx.BindAndValidate(c, &req)`, que internamente usa `apperror.FromValidationError` (mensajes traducidos al español) o `apperror.RequiredFieldsFrom` como fallback.
- Los mensajes de error están mezclados español/inglés (inconsistencia real, no intencional — ver bugs conocidos).

## Middleware (orden importa, ver `main.go`)

```
Recovery → RequestLogger → CORS → ErrorHandler → RateLimiter → rutas
```

`ErrorHandler` debe ir **antes** de las rutas pero después de CORS para que los errores también lleven headers CORS. `RateLimiter` usa un rate fijo `10-S` (10 req/seg) por IP, en memoria (no distribuido — se resetea por instancia, no sirve si se escala horizontalmente).

Auth: `middleware.AuthRequired(jwtSecret)` valida `Authorization: Bearer <token>`, decodifica el claim `sub` y lo guarda en el contexto Gin como `c.Set("userID", <string>)`. Los handlers protegidos leen `c.Get("userID")` y parsean a `uuid.UUID`.

## Modelos de dominio

- **User** (`internal/model/user`): UUID PK, email único, password hasheado, soft delete (`gorm.DeletedAt`).
- **File** (`internal/model/file`): UUID PK, pertenece a un `UserID`, guarda `BucketName` + `ObjectKey` (path en MinIO = `"<userID>/<uuid>"`), `Status` (pending/uploaded/failed/deleted), soft delete. Tiene `Width`/`Height`/`Checksum` en el schema pero **no se usan todavía** (no hay procesamiento de imágenes ni cálculo de checksum implementado).
- **Folder** (`internal/model/folder`) — **completo** (CRUD implementado, ver sección "Feature: carpetas (folders)" más abajo). Permite organizar archivos en carpetas jerárquicas (`ParentID *uuid.UUID` para árbol de carpetas, `nil` = raíz).

## Feature: carpetas (folders)

CRUD completo de carpetas ligadas a archivos, implementado siguiendo el patrón de capas del proyecto:

- `internal/model/folder/folder.go`: `Folder{ID, UserID, ParentID *uuid.UUID, Name, CreatedAt, UpdatedAt, DeletedAt}`. `ParentID` nil = carpeta raíz.
- `internal/model/file/file.go`: se agregó `FolderID *uuid.UUID` (nil = archivo suelto en la raíz del usuario, sin carpeta).
- `internal/store/folder_store.go` (`FolderStore`): `Create`, `GetByID`, `ListByUserID(userID, parentID)`, `ExistsByUserParentAndName` (para evitar nombres duplicados dentro del mismo padre), `HasChildren`, `Update`, `Delete`.
- `internal/store/file_store.go`: `ListByUserID` ahora acepta `folderID *uuid.UUID` opcional (si es `nil` no filtra, mantiene compatibilidad con el comportamiento previo). Se agregó `CountByFolderID` para saber si una carpeta tiene archivos.
- `internal/service/folder_service.go` (`FolderService`): reglas de negocio —
  - Ownership: toda operación verifica que la carpeta pertenezca al `userID` del JWT (`getOwned`), devolviendo `404 NotFound` (no `403`) si es de otro usuario, para no filtrar existencia.
  - Nombres únicos por (userID, parentID) — no se pueden crear/renombrar/mover dos carpetas con el mismo nombre en el mismo padre.
  - `Move` previene ciclos: no se puede mover una carpeta dentro de sí misma ni de una de sus propias subcarpetas (`assertNotDescendant`, camina el árbol hacia arriba con límite de profundidad `maxFolderDepth = 100` como salvaguarda).
  - `Delete` es **no destructivo por defecto**: si la carpeta tiene subcarpetas o archivos, devuelve `409 Conflict` en vez de borrar en cascada. Decisión deliberada (KISS/seguridad): evita borrar fotos del usuario por accidente vía cascada; si en el futuro se quiere borrado recursivo, debe ser una acción explícita y separada (ej. `?recursive=true`), no el comportamiento por defecto.
- `internal/handler/folder/` (`FolderHandler` + `dto.go` + `router.go`), montado bajo `/folders` con `AuthRequired`:
  - `POST /folders` — crear (`name` requerido, `parentId` opcional).
  - `GET /folders?parentId=` — listar (sin `parentId` = carpetas raíz).
  - `GET /folders/:id` — obtener una carpeta.
  - `PATCH /folders/:id` — renombrar (`name`).
  - `PATCH /folders/:id/move` — mover (`parentId` opcional, vacío = mover a raíz). Se separó de renombrar para evitar la ambigüedad de JSON entre "campo ausente" vs "campo null" en `parentId`.
  - `DELETE /folders/:id` — eliminar (falla con 409 si no está vacía).
- Integración con archivos: `POST /files/upload` acepta un campo de formulario opcional `folderId`; `GET /files/list` acepta `?folderId=` opcional para filtrar. `FileService.Upload` valida que la carpeta exista y pertenezca al usuario antes de subir a MinIO.
- **Nota importante sobre MinIO**: MinIO no tiene carpetas reales (es almacenamiento de objetos planos). El concepto de "carpeta" es puramente relacional en Postgres vía `Folder`/`File.FolderID` — el `ObjectKey` en el bucket sigue siendo plano (`"<userID>/<uuid>"`), no refleja la ruta de carpetas. Esto es intencional y no un bug.
- Se extrajo `httpx.UserIDFromContext(c) (uuid.UUID, error)` (en `internal/httpx/context.go`) porque el patrón de leer/parsear `userID` del contexto Gin ya se repetía en `file_handler.go` y se iba a repetir en `folder_handler.go` — usado ahora en ambos handlers.

## Feature: descarga y borrado de archivos

Antes de esto, la API podía subir y listar metadata de archivos pero **no había forma de ver ni borrar un archivo** — el gap más grave para una app de fotos. Implementado en `feature/file-download-delete` (a partir de `main` ya con folders+swagger mergeados):

- `internal/storage/minio.go`: `Connect` ahora verifica con `BucketExists` si el bucket de `cfg.MinioBucket` existe al arrancar, y si no, lo crea con `MakeBucket`. Antes solo abría el cliente sin validar nada, y el primer upload fallaba en runtime con un error poco claro si el bucket no existía.
- `internal/service/file_service.go` (`FileService`): se agregaron `GetByID`, `DownloadURL`, `Delete`.
  - `GetByID`/`DownloadURL` verifican ownership igual que en `folder_service.go` (404 si el archivo no es del usuario).
  - `DownloadURL` genera una URL firmada de MinIO (`PresignedGetObject`, expira en 15 min vía `downloadURLExpiry`) — el cliente (Next.js/Flutter) la usa directo como `src` de imagen/descarga, sin proxying de bytes a través del backend Go.
  - `Delete` primero hace `RemoveObject` en MinIO y **solo si eso tiene éxito** hace soft-delete del registro en Postgres (`fileStore.Delete`) — ese orden es deliberado: si se borrara la fila primero y `RemoveObject` fallara después, quedaría un objeto huérfano en MinIO sin metadata que lo referencie (más difícil de limpiar que el caso inverso).
- `internal/handler/file/` — nuevas rutas, todas bajo `AuthRequired`:
  - `GET /files/:id` — metadata de un archivo.
  - `GET /files/:id/url` — `{"url": "..."}`, URL firmada válida 15 minutos.
  - `DELETE /files/:id` — borra el objeto de MinIO + soft-delete en DB, `204` en éxito.
- Probado end-to-end manualmente (registro → login → upload → get → url → fetch real del contenido vía la URL firmada → delete → get devuelve 404): funciona correctamente.
- Sigue pendiente (no bloqueante): subida vía presigned PUT (hoy el upload sigue proxificando el archivo completo a través del backend con `PutObject`, ver deuda técnica #9 abajo).

## Bugs / deuda técnica conocida (para el agente Go+Gin y los reviewers)

1. **Sin tests**: no existe ni un solo `_test.go` en el repo (incluye folders y el flujo de descarga/borrado de archivos).
2. **Sin paginación defensiva**: `FileHandler.List` no valida límites superiores de `limit`/`offset` (podría pedirse `limit=999999999`).
3. **CORS hardcodeado** a `http://localhost:4000` — cuando se conecte el frontend Next.js real habrá que mover esto a `config.Config` (variable de entorno) en vez de estar fijo en `middleware/cors.go`.
4. **Rate limiter en memoria**: no sirve si se corren múltiples réplicas del servidor (no hay backend compartido tipo Redis).
5. **`.env.example` incompleto**: falta documentar `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET` (sí están en `config.Config` pero no en el ejemplo).
6. **Health check superficial**: `GetHealthStatus` solo responde 200 fijo, no verifica conexión real a DB ni a MinIO.
7. **Mensajes inconsistentes**: mezcla de español ("Archivo .env no econctrado" con typo, "Este campo es obligatorio") e inglés ("no file provided", "invalid credentials") en errores/logs.
8. **Sin borrado en cascada de carpetas**: `DELETE /folders/:id` rechaza con 409 si la carpeta tiene subcarpetas o archivos (ver sección "Feature: carpetas" arriba) — decisión intencional, no un bug, pero puede sorprender si se espera borrado recursivo. Con el nuevo `DELETE /files/:id` ya es posible vaciar una carpeta desde la API antes de borrarla.
9. **Sin presigned PUT para upload**: la descarga ya usa URL firmada (`DownloadURL`), pero la subida (`POST /files/upload`) sigue proxificando el archivo completo a través del backend Go — no escala bien para archivos grandes de foto/video. Considerar `PresignedPutObject` a futuro si el volumen lo justifica.

## Qué NO cambiar sin preguntar

- El patrón interfaz-primero en `store`/`service` (facilita testing futuro).
- El flujo `c.Error(...)` + `ErrorHandler` centralizado — no introducir manejo de errores ad-hoc en handlers.
- El orden del middleware stack en `main.go`.

## Roadmap mencionado por el usuario

- Completar CRUD de carpetas (folder) sobre MinIO — trabajo en curso ahora mismo en la rama `feature/crud-folder-on-minio`.
- Conectar un frontend en Next.js.
- Conectar una app Android hecha en Flutter.
- Ambos clientes consumen la misma API REST — mantener contratos JSON estables y documentados a medida que crecen.
