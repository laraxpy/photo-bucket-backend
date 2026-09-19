# Guía de integración — Photo Bucket Backend API

Documento de referencia para el agente/equipo que construya el frontend (Next.js) o la app (Flutter) que consuma esta API. Describe el contrato HTTP real tal como está implementado hoy en `main`, no un diseño aspiracional.

## 1. Lo esencial

- **Base URL (dev)**: `http://localhost:3001` (puerto configurable vía `PORT` en el `.env` del backend).
- **Formato**: JSON en todos lados, excepto el upload de archivos (`multipart/form-data`).
- **Auth**: JWT Bearer + refresh token. No hay cookies — el cliente es responsable de guardar ambos tokens y usar `/user/refresh` para renovar la sesion cuando el access token expire (24hs).
- **Documentación interactiva viva**: `GET /swagger/index.html` (UI) y `GET /swagger/doc.json` (spec OpenAPI 2.0 crudo — se puede importar a Postman/Insomnia o usar para generar un cliente tipado, ej. `openapi-typescript`).
- **CORS**: el backend solo acepta requests desde los orígenes listados en la variable de entorno `CORS_ALLOWED_ORIGINS` del backend (default: `http://localhost:4000`). **Si el frontend corre en otro puerto/dominio, hay que pedir que se agregue ahí** — el navegador va a bloquear las requests silenciosamente si no está.

## 2. Autenticación

### Registrar usuario
```
POST /user/register
Content-Type: application/json

{
  "name": "Ana",       // requerido, 3-50 caracteres
  "email": "a@b.com",  // requerido, formato email
  "password": "..."    // requerido, 8-50 caracteres
}
```
`201 Created` → devuelve el `User` creado (sin password). `409 Conflict` si el email ya existe.

### Login
```
POST /user/login
Content-Type: application/json

{ "email": "a@b.com", "password": "..." }
```
`200 OK` → `{ "token": "<jwt>", "refreshToken": "<opaque-string>" }`. `401 Unauthorized` si las credenciales son incorrectas (mismo error tanto si el email no existe como si la password es incorrecta — no se filtra cuál de las dos falló).

### Usar el token
Todas las rutas de `files` y `folders` requieren:
```
Authorization: Bearer <token>
```
El JWT (`token`) expira a las **24 horas** de emitido (claim `exp`). Si expira, cualquier request protegida devuelve `401`.

### Renovar sesion
```
POST /user/refresh
Content-Type: application/json

{ "refreshToken": "<opaque-string>" }
```
`200 OK` → `{ "token": "<jwt>", "refreshToken": "<opaque-string>" }` — un par de tokens **nuevo**. `401 Unauthorized` si el refresh token es invalido, ya expiró (dura 30 días), o ya fue usado.

**El refresh token rota en cada uso**: cada llamada a `/user/refresh` invalida el refresh token recibido y devuelve uno nuevo. El cliente tiene que descartar el anterior y guardar el nuevo — reusar un refresh token ya canjeado devuelve `401`. Si el refresh token expira o se invalida, hay que loguear de nuevo.

## 3. Modelo `User`
```json
{
  "id": "uuid",
  "name": "string",
  "email": "string",
  "isActive": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z",
  "deletedAt": null
}
```

## 4. Carpetas (`/folders`)

Las carpetas son un árbol por usuario (metadata en Postgres). **MinIO no tiene carpetas reales** — esto es puramente organizativo, no cambia cómo se guardan los archivos.

### Modelo `Folder`
```json
{
  "id": "uuid",
  "userId": "uuid",
  "parentId": "uuid | null",
  "name": "string",
  "createdAt": "...",
  "updatedAt": "...",
  "deletedAt": null
}
```

### Endpoints

| Método | Ruta | Body / Query | Notas |
|---|---|---|---|
| POST | `/folders` | `{ "name": string, "parentId"?: string }` | `parentId` vacío u omitido = carpeta raíz. `201`. `409` si ya existe una carpeta con ese nombre en el mismo padre. `404` si `parentId` no existe o no es tuyo. |
| GET | `/folders?parentId=` | — | Sin `parentId` = carpetas raíz. Con `parentId` = subcarpetas de esa carpeta. `200` con array (puede ser `[]`). |
| GET | `/folders/:id` | — | `404` si no existe o no es tuya. |
| PATCH | `/folders/:id` | `{ "name": string }` | Solo renombra. `409` si el nuevo nombre choca con otra carpeta del mismo padre. |
| PATCH | `/folders/:id/move` | `{ "parentId"?: string }` | Solo mueve. `parentId` vacío/omitido = mover a raíz. **Está separado de renombrar a propósito** — es un endpoint distinto, no un campo más del PATCH normal. `400` si se intenta mover una carpeta dentro de sí misma o de una de sus propias subcarpetas (previene ciclos). |
| DELETE | `/folders/:id` | — | `204` si se borró. **`409` si la carpeta tiene subcarpetas o archivos adentro** — hay que vaciarla primero (mover/borrar su contenido). No hay borrado en cascada. |

## 5. Archivos (`/files`)

### Modelo `File`
```json
{
  "id": "uuid",
  "userId": "uuid",
  "folderId": "uuid | null",
  "bucketName": "string",
  "objectKey": "string",
  "etag": "string",
  "originalName": "string",
  "contentType": "string",
  "sizeBytes": 12345,
  "width": 0,
  "height": 0,
  "thumbnailSmallObjectKey": "string | ausente",
  "thumbnailMediumObjectKey": "string | ausente",
  "status": "pending | uploaded | failed | deleted",
  "isPublic": false,
  "checksum": "",
  "createdAt": "...",
  "updatedAt": "...",
  "deletedAt": null
}
```
`width`, `height` y `checksum` existen en el modelo pero **no se calculan todavía** (siempre vienen en 0/vacío) — no confiar en esos campos hoy.

`thumbnailSmallObjectKey`/`thumbnailMediumObjectKey` vienen ausentes (omitidos del JSON) si el archivo no tiene esa variante generada. **No usar estos campos directamente** — son un detalle interno de almacenamiento; para mostrar la miniatura, usar siempre el endpoint `GET /files/:id/thumbnail-url` de la sección siguiente.

### Subir un archivo
```
POST /files/upload
Content-Type: multipart/form-data

file: <binario>          // requerido
folderId: "uuid"          // opcional, form field (no query param). Vacío/omitido = raíz.
```
`201` → el `File` creado. `404` si `folderId` no existe o no es tuya.

Se puede subir cualquier tipo de archivo (no hay una lista blanca de `contentType`). El backend genera automáticamente **dos variantes de miniatura** para:
- Imágenes: `image/jpeg`, `image/png`, `image/gif`.
- Video: `video/mp4` (un frame extraído a 0.5s del video, redimensionado igual que una foto). Por ahora es el único formato de video con miniatura — otros formatos de video (mov, webm, etc.) se suben bien, pero sin miniatura.

Las dos variantes:
- **`small`** (máx. 200x200px): para grillas/listados de la galería.
- **`medium`** (máx. 800x800px): para el photo viewer (vista ampliada, no de pantalla completa del original).

Para cualquier otro `contentType`, o si la generación falla por algún motivo, el archivo se sube igual — simplemente no vas a tener esas miniaturas y `GET /files/:id/thumbnail-url` va a hacer fallback (ver más abajo).

### Listar archivos
```
GET /files/list?folderId=&limit=20&offset=0
```
- `folderId` opcional: si se omite, trae solo los archivos de la **raíz** (sin carpeta); si se pasa, filtra por esa carpeta puntual.
- `limit`: entero 1-100 (default 20). Fuera de rango → `400`.
- `offset`: entero ≥ 0 (default 0). Negativo → `400`.
- `200` → array de `File` (puede ser `[]`).

### Ver metadata de un archivo
```
GET /files/:id
```
`200` → el `File`. `404` si no existe o no es tuyo.

### Obtener una URL para mostrar/descargar el archivo
```
GET /files/:id/url
```
`200` → `{ "url": "https://..." }`.

**Esto es lo que hay que usar como `src` de una imagen o link de descarga** — es una URL firmada de MinIO, **válida por 15 minutos**. No se puede cachear indefinidamente: si el usuario deja la página abierta más de 15 minutos y la imagen se recarga, hay que volver a pedir la URL. Para una galería, lo más simple es pedir la URL justo antes de renderizar cada imagen (o refrescarla si falla la carga).

### Obtener una URL para la miniatura

```
GET /files/:id/thumbnail-url?size=small|medium
```
`size` es opcional, default `small`. `400` si se manda un valor que no sea `small` ni `medium`.

`200` → `{ "url": "https://..." }` — misma mecánica que `/files/:id/url` (URL firmada, válida 15 minutos), pero apuntando a la miniatura pedida.

**Uso recomendado por pantalla**:
- **Grilla/listado de la galería** → `?size=small` (o sin `size`, es el default).
- **Photo viewer** (vista ampliada de una foto) → `?size=medium`.
- **Descarga del archivo por el cliente** → `GET /files/:id/url` (el original, sin este endpoint).

**Fallback en cadena, siempre es seguro llamarlo**: si pedís `medium` y no existe, cae a `small`; si esa tampoco existe (archivo sin ninguna miniatura — tipo no soportado, o falló la generación), cae al original. Nunca vas a recibir un error por falta de miniatura, siempre una URL usable.

### Borrar un archivo
```
DELETE /files/:id
```
`204` si se borró (borra el objeto real de MinIO, no es reversible). `404` si no existe o no es tuyo.

### Descarga masiva (zip)
```
POST /files/download-zip
Content-Type: application/json

{
  "fileIds": ["uuid", "..."],   // opcional
  "folderId": "uuid"             // opcional
}
```
Hay que mandar `fileIds`, `folderId`, o ambos — el resultado es la unión de los dos (sin duplicados si un archivo está en los dos conjuntos). `folderId` trae **todos** los archivos de esa carpeta puntual (no recursivo a subcarpetas).

`200` → el archivo `.zip` en streaming (`Content-Type: application/zip`, `Content-Disposition: attachment`), con los archivos originales (no las miniaturas) usando su `originalName` como nombre dentro del zip. Si dos archivos seleccionados tienen el mismo nombre (ej. la misma foto subida a dos carpetas distintas), el segundo se renombra dentro del zip como `nombre (1).ext`, `nombre (2).ext`, etc. para no pisarse.

`400` si no se manda `fileIds` ni `folderId` (o ninguno de los dos trae archivos), o si algún id no es un UUID válido. `404` si algún `fileId` no existe o no es tuyo — **esto se valida antes de generar el zip**, así que un id inválido nunca te deja con una descarga a medias por ese motivo.

**Importante para el cliente**: como el tamaño de la descarga depende de cuántos/qué tan grandes son los archivos pedidos (no hay límite de cantidad), tratá esta request como una descarga de archivo normal (dejar que el navegador/cliente HTTP maneje el stream a un archivo, no intentar cargar la respuesta completa en memoria antes de guardarla) y no le pongas un timeout corto — puede tardar bastante con selecciones grandes.

## 6. Formato de error (uniforme en toda la API)

Cualquier error, de cualquier endpoint, tiene esta forma:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation error",
    "fields": { "email": "Este campo es obligatorio" }
  }
}
```
`fields` solo aparece en errores de validación (`400` con `VALIDATION_ERROR`). Los `code` posibles:

| Code | HTTP status | Cuándo |
|---|---|---|
| `BAD_REQUEST` | 400 | Parámetro inválido (uuid mal formado, límites de paginación, etc.) |
| `VALIDATION_ERROR` | 400 | Falla `binding` de un DTO (body de request) |
| `UNAUTHORIZED` | 401 | Falta token, token inválido/expirado, o credenciales de login incorrectas |
| `FORBIDDEN` | 403 | (definido pero no usado activamente hoy) |
| `NOT_FOUND` | 404 | Recurso no existe **o pertenece a otro usuario** (mismo código para ambos casos, a propósito — no se filtra cuál es) |
| `CONFLICT` | 409 | Nombre de carpeta duplicado, carpeta no vacía al borrar, email duplicado al registrar |
| `TOO_MANY_REQUEST` | 429 | Rate limit superado |
| `NO_METHOD` | 405 | Método HTTP no soportado en esa ruta |
| `INTERNAL_ERROR` | 500 | Error no esperado del servidor |

**Importante para el manejo de errores del frontend**: un `404` en `/files/:id` o `/folders/:id` significa "no existe o no es tuyo" — no asumas que el recurso nunca existió, podría ser de otro usuario. No hace falta (ni se puede) distinguir los dos casos desde el cliente.

## 7. Límites a tener en cuenta

- **Rate limit**: 30 requests/segundo por IP (in-memory, un solo proceso — no aplica bien si el backend corre en múltiples réplicas, pero para dev/single-instance está activo; configurable vía `RATE_LIMIT` en el `.env` del backend, formato `"<n>-S"`). Pasado el límite: `429 TOO_MANY_REQUEST`.
- **Sin websockets/tiempo real**: todo es request/response. Si el frontend necesita "el archivo terminó de subir" en tiempo real para múltiples pestañas, no hay push del backend — hay que hacer polling manual.
- **Upload síncrono**: `POST /files/upload` sube el archivo completo al backend y de ahí a MinIO (no hay presigned PUT todavía) — para archivos grandes, esperar que la request tarde proporcionalmente al tamaño y al ancho de banda del servidor, no es instantáneo ni paralelizable desde el cliente. Los videos, al pesar más que una foto típica, van a tardar sensiblemente más en subir — el request no responde hasta que termina de subirse el original **y** de generarse las miniaturas. El servidor tolera hasta 60s por request para esto (fotos de 40-50MP de celulares modernos entran cómodas); pasado ese tiempo, la conexión se corta y el cliente ve un error de red o `500`.
- **`POST /files/upload` es de a un archivo por request** — no hay endpoint de batch. Para subir varios archivos a la vez (ej. una galería completa), ver la sección siguiente.

### Subir varios archivos a la vez (evitando el 429)

Como el rate limit es por IP y `/files/upload` es un endpoint por archivo, disparar todas las subidas en paralelo sin control (`Promise.all` de N fotos) puede superar el límite y devolver `429` en varias de ellas. La forma recomendada de integrarlo:

1. **Cola con concurrencia acotada**: subir de a 3-4 archivos en simultáneo como máximo (no todos a la vez), encolando el resto. Cualquier librería de cola (`p-limit`, `p-queue` en JS, o un semáforo casero) sirve.
2. **Progreso por archivo**: cada request de `POST /files/upload` es un `multipart/form-data` normal — usá el evento de progreso nativo del cliente HTTP (`onUploadProgress` en axios, o `XMLHttpRequest.upload.onprogress`) para saber cuántos bytes de *ese* archivo ya se enviaron.
3. **Progreso agregado (la barra de "43%" de la galería completa)**: se calcula 100% en el cliente, no lo devuelve el backend. Por ejemplo: `(archivos completados + progreso_bytes_del_archivo_actual / tamaño_del_archivo_actual) / total_archivos`.
4. **Reintentos ante `429`**: si igual llega un `429` (ráfaga), esperar un instante corto (ej. 500ms-1s) y reintentar ese archivo puntual, no todo el batch.

## 8. Flujo típico end-to-end

```
1. POST /user/register              → crear cuenta
2. POST /user/login                 → guardar token + refreshToken (localStorage, memoria, etc. — decisión del frontend)
3. POST /folders {"name":"Viajes"}  → crear una carpeta (opcional)
4. POST /files/upload (folderId=…)  → subir una foto
5. GET  /files/list?folderId=…      → listar lo subido
6. GET  /files/:id/thumbnail-url?size=small   → miniatura para la grilla de la galería
6b. GET /files/:id/thumbnail-url?size=medium  → vista ampliada en el photo viewer
6c. GET /files/:id/url                        → el original (solo al descargar una foto puntual)
7. DELETE /files/:id                → borrar si hace falta
8. POST /user/refresh {"refreshToken":…} → cuando el token expira (401), canjear el refreshToken por un par nuevo
```

## 9. Qué NO existe todavía (no asumir)

- Recuperación de contraseña / cambio de email.
- Compartir archivos o carpetas entre usuarios (`isPublic` existe en el modelo pero no tiene ningún efecto real hoy).
- Metadata real de imagen (`width`/`height`) — las miniaturas ya existen (ver sección 5), pero esos dos campos del modelo `File` siguen sin calcularse.
- Búsqueda de archivos por nombre.
- Logout / revocación manual de un refresh token (revocar todos los tokens de un usuario, invalidar sesiones desde otro dispositivo, etc.).
