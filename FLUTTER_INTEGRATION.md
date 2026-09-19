# Guía de integración — Photo Bucket Backend API (Flutter)

Documento de referencia para desarrollar la app Flutter que consume esta API. Describe el contrato HTTP real tal como está implementado hoy en `main`, con ejemplos de código en Dart usando `dio` (el cliente HTTP recomendado para este caso: soporta interceptors, multipart, progreso de subida/descarga y cancelación de requests, todo lo que se necesita acá).

Para el contrato equivalente pensado para un frontend web (JS/axios), ver [FRONTEND_INTEGRATION.md](FRONTEND_INTEGRATION.md) — el contrato HTTP es el mismo, solo cambian los ejemplos de cliente.

## 1. Lo esencial

- **Base URL (dev)**: `http://localhost:3001` en un emulador Android hay que usar `http://10.0.2.2:3001` en su lugar (`localhost` en el emulador apunta al propio emulador, no a tu máquina). En iOS Simulator, `http://localhost:3001` funciona directo. En un dispositivo físico, la IP de tu máquina en la red local.
- **Formato**: JSON en todos lados, excepto el upload de archivos (`multipart/form-data`).
- **Auth**: JWT Bearer + refresh token. El cliente guarda ambos tokens y usa `/user/refresh` para renovar la sesión cuando el access token expira (24hs).
- **CORS no aplica** a una app Flutter nativa (Android/iOS/desktop) — esa restricción es del navegador. Si además vas a correr **Flutter Web**, ahí sí aplica igual que a cualquier frontend web: hay que pedir que se agregue tu origen a `CORS_ALLOWED_ORIGINS` en el backend.
- **Documentación interactiva viva**: `GET /swagger/index.html` (UI) y `GET /swagger/doc.json` (spec OpenAPI 2.0 — se puede usar para generar un cliente Dart tipado con `openapi-generator` si preferís eso a los servicios manuales de esta guía).

### Dependencias sugeridas (`pubspec.yaml`)
```yaml
dependencies:
  dio: ^5.4.0                    # cliente HTTP
  flutter_secure_storage: ^9.0.0 # guardar token/refreshToken de forma segura
  pool: ^1.5.1                   # limitar concurrencia al subir varios archivos
  cached_network_image: ^3.3.0   # mostrar miniaturas con cache (opcional pero recomendado)
  path_provider: ^2.1.0          # ubicar carpetas de descarga/temp
  image_picker: ^1.0.0           # elegir fotos/videos para subir (opcional)
```

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
`200 OK` → `{ "token": "<jwt>", "refreshToken": "<opaque-string>" }`. `401 Unauthorized` si las credenciales son incorrectas (mismo error tanto si el email no existe como si la password es incorrecta).

### Usar el token
Todas las rutas de `files` y `folders` requieren:
```
Authorization: Bearer <token>
```
El JWT (`token`) expira a las **24 horas** de emitido. Si expira, cualquier request protegida devuelve `401`.

### Renovar sesión
```
POST /user/refresh
Content-Type: application/json

{ "refreshToken": "<opaque-string>" }
```
`200 OK` → `{ "token": "<jwt>", "refreshToken": "<opaque-string>" }` — un par de tokens **nuevo**. `401 Unauthorized` si el refresh token es inválido, ya expiró (dura 30 días), o ya fue usado.

**El refresh token rota en cada uso**: cada llamada a `/user/refresh` invalida el token recibido y devuelve uno nuevo. Guardá siempre el nuevo y descartá el anterior — reusar uno ya canjeado devuelve `401`.

### Guardar los tokens de forma segura

`flutter_secure_storage` usa Keychain en iOS y EncryptedSharedPreferences/Keystore en Android — es lo correcto acá, nunca `SharedPreferences` en texto plano para esto.

```dart
class TokenStorage {
  static const _accessTokenKey = 'access_token';
  static const _refreshTokenKey = 'refresh_token';
  final _storage = const FlutterSecureStorage();

  Future<void> saveTokens(String accessToken, String refreshToken) async {
    await _storage.write(key: _accessTokenKey, value: accessToken);
    await _storage.write(key: _refreshTokenKey, value: refreshToken);
  }

  Future<String?> getAccessToken() => _storage.read(key: _accessTokenKey);
  Future<String?> getRefreshToken() => _storage.read(key: _refreshTokenKey);

  Future<void> clear() async {
    await _storage.delete(key: _accessTokenKey);
    await _storage.delete(key: _refreshTokenKey);
  }
}
```

### Cliente Dio con refresh automático (interceptor)

Este es el patrón central de toda la integración: adjunta el token a cada request, y ante un `401` intenta refrescar la sesión **una sola vez** (aunque varios requests fallen al mismo tiempo — ej. varias fotos pidiendo su URL firmada), reintentando el/los requests originales con el token nuevo. Si el refresh también falla, recién ahí desloguea.

```dart
class ApiClient {
  final Dio dio;
  final TokenStorage _tokenStorage;
  Completer<void>? _refreshCompleter;

  ApiClient(this._tokenStorage, {required String baseUrl})
      : dio = Dio(BaseOptions(baseUrl: baseUrl)) {
    dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        final token = await _tokenStorage.getAccessToken();
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
      onError: (DioException err, handler) async {
        final isAuthRoute = err.requestOptions.path.contains('/user/login') ||
            err.requestOptions.path.contains('/user/refresh');
        if (err.response?.statusCode != 401 || isAuthRoute) {
          return handler.next(err);
        }

        try {
          await _refreshSession(); // espera si ya hay un refresh en curso
        } catch (_) {
          await _tokenStorage.clear();
          // TODO: navegar a la pantalla de login
          return handler.next(err);
        }

        // reintenta el request original con el token nuevo
        final token = await _tokenStorage.getAccessToken();
        final opts = err.requestOptions;
        opts.headers['Authorization'] = 'Bearer $token';
        try {
          final response = await dio.fetch(opts);
          return handler.resolve(response);
        } catch (e) {
          return handler.next(err);
        }
      },
    ));
  }

  /// Si ya hay un refresh en curso, espera ese resultado en vez de disparar
  /// uno nuevo — el refresh token rota, así que dos refreshes en paralelo
  /// harían que el segundo falle con el token ya invalidado por el primero.
  Future<void> _refreshSession() {
    if (_refreshCompleter != null) return _refreshCompleter!.future;

    final completer = Completer<void>();
    _refreshCompleter = completer;

    () async {
      try {
        final refreshToken = await _tokenStorage.getRefreshToken();
        final response = await dio.post('/user/refresh', data: {'refreshToken': refreshToken});
        await _tokenStorage.saveTokens(response.data['token'], response.data['refreshToken']);
        completer.complete();
      } catch (e) {
        completer.completeError(e);
      } finally {
        _refreshCompleter = null;
      }
    }();

    return completer.future;
  }
}
```

## 3. Modelo `User`
```dart
class User {
  final String id;
  final String name;
  final String email;
  final bool isActive;

  User({required this.id, required this.name, required this.email, required this.isActive});

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: json['id'],
        name: json['name'],
        email: json['email'],
        isActive: json['isActive'] ?? true,
      );
}
```

## 4. Carpetas (`/folders`)

Las carpetas son un árbol por usuario (metadata en Postgres). **MinIO no tiene carpetas reales** — esto es puramente organizativo, no cambia cómo se guardan los archivos.

### Modelo `Folder`
```dart
class Folder {
  final String id;
  final String userId;
  final String? parentId;
  final String name;

  Folder({required this.id, required this.userId, this.parentId, required this.name});

  factory Folder.fromJson(Map<String, dynamic> json) => Folder(
        id: json['id'],
        userId: json['userId'],
        parentId: json['parentId'],
        name: json['name'],
      );
}
```

### Endpoints

| Método | Ruta | Body / Query | Notas |
|---|---|---|---|
| POST | `/folders` | `{ "name": string, "parentId"?: string }` | `parentId` vacío u omitido = carpeta raíz. `201`. `409` si ya existe una carpeta con ese nombre en el mismo padre. `404` si `parentId` no existe o no es tuyo. |
| GET | `/folders?parentId=` | — | Sin `parentId` = carpetas raíz. Con `parentId` = subcarpetas de esa carpeta. `200` con array (puede ser `[]`). |
| GET | `/folders/:id` | — | `404` si no existe o no es tuya. |
| PATCH | `/folders/:id` | `{ "name": string }` | Solo renombra. `409` si el nuevo nombre choca con otra carpeta del mismo padre. |
| PATCH | `/folders/:id/move` | `{ "parentId"?: string }` | Solo mueve. `parentId` vacío/omitido = mover a raíz. Endpoint separado de renombrar a propósito. `400` si se intenta mover una carpeta dentro de sí misma o de una de sus propias subcarpetas (previene ciclos). |
| DELETE | `/folders/:id` | — | `204` si se borró. **`409` si la carpeta tiene subcarpetas o archivos adentro** — hay que vaciarla primero. No hay borrado en cascada. |

```dart
class FolderService {
  final Dio _dio;
  FolderService(this._dio);

  Future<Folder> create(String name, {String? parentId}) async {
    final res = await _dio.post('/folders', data: {'name': name, if (parentId != null) 'parentId': parentId});
    return Folder.fromJson(res.data);
  }

  Future<List<Folder>> list({String? parentId}) async {
    final res = await _dio.get('/folders', queryParameters: {if (parentId != null) 'parentId': parentId});
    return (res.data as List).map((e) => Folder.fromJson(e)).toList();
  }
}
```

## 5. Archivos (`/files`)

### Modelo `File`
```dart
class PhotoFile {
  final String id;
  final String userId;
  final String? folderId;
  final String originalName;
  final String contentType;
  final int sizeBytes;
  final String status; // pending | uploaded | failed | deleted
  final bool hasThumbnail; // derivado: thumbnailSmallObjectKey u otro presente

  PhotoFile({
    required this.id,
    required this.userId,
    this.folderId,
    required this.originalName,
    required this.contentType,
    required this.sizeBytes,
    required this.status,
    required this.hasThumbnail,
  });

  factory PhotoFile.fromJson(Map<String, dynamic> json) => PhotoFile(
        id: json['id'],
        userId: json['userId'],
        folderId: json['folderId'],
        originalName: json['originalName'],
        contentType: json['contentType'],
        sizeBytes: json['sizeBytes'],
        status: json['status'],
        hasThumbnail: json['thumbnailSmallObjectKey'] != null || json['thumbnailMediumObjectKey'] != null,
      );
}
```
`width`, `height` y `checksum` existen en el modelo del backend pero **no se calculan todavía** — no los mapees ni confíes en ellos. Los campos `thumbnail*ObjectKey` son un detalle interno de almacenamiento: no los uses directo para mostrar nada, siempre pedí la URL vía `GET /files/:id/thumbnail-url` (más abajo).

### Subir un archivo
```
POST /files/upload
Content-Type: multipart/form-data

file: <binario>          // requerido
folderId: "uuid"          // opcional, form field. Vacío/omitido = raíz.
```
`201` → el `File` creado. `404` si `folderId` no existe o no es tuya.

Si el archivo subido es una imagen (`image/jpeg`, `image/png`, `image/gif`) o un video `video/mp4`, el backend genera automáticamente **dos miniaturas**: `small` (máx. 200x200px, para listados) y `medium` (máx. 800x800px, para el photo viewer). Para video, la miniatura es un frame extraído a los 0.5s. Cualquier otro tipo de archivo se sube igual, simplemente sin miniaturas.

```dart
class FileService {
  final Dio _dio;
  FileService(this._dio);

  Future<PhotoFile> upload(
    File file, {
    String? folderId,
    void Function(int sent, int total)? onProgress,
  }) async {
    final formData = FormData.fromMap({
      'file': await MultipartFile.fromFile(file.path, filename: file.path.split('/').last),
      if (folderId != null) 'folderId': folderId,
    });
    final res = await _dio.post('/files/upload', data: formData, onSendProgress: onProgress);
    return PhotoFile.fromJson(res.data);
  }
}
```

### Listar archivos
```
GET /files/list?folderId=&limit=20&offset=0
```
- `folderId` opcional: si se omite, trae solo los archivos de la **raíz** (sin carpeta); si se pasa, filtra por esa carpeta puntual.
- `limit`: entero 1-100 (default 20). `offset`: entero ≥ 0 (default 0).
- `200` → array de `File` (puede ser `[]`).

```dart
Future<List<PhotoFile>> list({String? folderId, int limit = 20, int offset = 0}) async {
  final res = await _dio.get('/files/list', queryParameters: {
    if (folderId != null) 'folderId': folderId,
    'limit': limit,
    'offset': offset,
  });
  return (res.data as List).map((e) => PhotoFile.fromJson(e)).toList();
}
```

### Mostrar una miniatura o el original
```
GET /files/:id/thumbnail-url?size=small|medium   → { "url": "https://..." }
GET /files/:id/url                                → { "url": "https://..." }  (el original)
```
`size` es opcional en `thumbnail-url`, default `small`. Ambas devuelven una **URL firmada de MinIO, válida por 15 minutos** — no requiere el header `Authorization` (ya viene autenticada en la propia URL), así que se puede usar directo en `Image.network` o `CachedNetworkImage`.

**Uso recomendado**: `size=small` para la grilla de la galería, `size=medium` para el photo viewer, `GET /files/:id/url` (el original) solo para descargar/compartir el archivo real. Si la miniatura pedida no existe (tipo no soportado, o falló la generación), el endpoint hace fallback automático — nunca falla por eso, siempre devuelve una URL usable.

```dart
Future<String> thumbnailUrl(String fileId, {String size = 'small'}) async {
  final res = await _dio.get('/files/$fileId/thumbnail-url', queryParameters: {'size': size});
  return res.data['url'];
}
```

```dart
// En un widget de galería:
CachedNetworkImage(
  imageUrl: await fileService.thumbnailUrl(file.id, size: 'small'),
  placeholder: (context, url) => const CircularProgressIndicator(),
  errorWidget: (context, url, error) => const Icon(Icons.broken_image),
)
```
Como la URL expira a los 15 minutos, no la guardes en el modelo/estado a largo plazo — pedila justo antes de renderizar, o volvé a pedirla si `errorWidget` se dispara.

### Borrar un archivo
```
DELETE /files/:id
```
`204` si se borró (irreversible). `404` si no existe o no es tuyo.

### Descarga masiva (zip)
```
POST /files/download-zip
Content-Type: application/json

{
  "fileIds": ["uuid", "..."],   // opcional
  "folderId": "uuid"             // opcional
}
```
Mandá `fileIds`, `folderId`, o ambos (unión, sin duplicados). `200` → el `.zip` en streaming. `400` si no seleccionaste nada. `404` si algún `fileId` no es tuyo — **se valida todo antes de generar el zip**, así que nunca te llega una descarga a medias por ese motivo.

Como no hay límite de cantidad de archivos, tratá esta descarga como podría tardar bastante — usá `path_provider` para bajarla a un archivo en disco en vez de cargarla completa en memoria si esperás selecciones grandes:

```dart
Future<File> downloadZip({List<String>? fileIds, String? folderId}) async {
  final response = await _dio.post(
    '/files/download-zip',
    data: {
      if (fileIds != null) 'fileIds': fileIds,
      if (folderId != null) 'folderId': folderId,
    },
    options: Options(responseType: ResponseType.bytes),
  );
  final dir = await getTemporaryDirectory();
  final file = File('${dir.path}/download_${DateTime.now().millisecondsSinceEpoch}.zip');
  await file.writeAsBytes(response.data);
  return file;
}
```
Para selecciones muy grandes, cambiá `ResponseType.bytes` por `ResponseType.stream` y escribí a disco con `response.data.stream.listen(...)` en vez de acumular todo en memoria antes de guardar.

## 6. Formato de error (uniforme en toda la API)

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation error",
    "fields": { "email": "Este campo es obligatorio" }
  }
}
```
`fields` solo aparece en errores de validación (`400` con `VALIDATION_ERROR`).

| Code | HTTP status | Cuándo |
|---|---|---|
| `BAD_REQUEST` | 400 | Parámetro inválido (uuid mal formado, límites de paginación, etc.) |
| `VALIDATION_ERROR` | 400 | Falla de validación del body de request |
| `UNAUTHORIZED` | 401 | Falta token, token inválido/expirado, o credenciales de login incorrectas |
| `FORBIDDEN` | 403 | (definido pero no usado activamente hoy) |
| `NOT_FOUND` | 404 | Recurso no existe **o pertenece a otro usuario** (mismo código para ambos casos, a propósito) |
| `CONFLICT` | 409 | Nombre de carpeta duplicado, carpeta no vacía al borrar, email duplicado al registrar |
| `TOO_MANY_REQUEST` | 429 | Rate limit superado |
| `NO_METHOD` | 405 | Método HTTP no soportado en esa ruta |
| `INTERNAL_ERROR` | 500 | Error no esperado del servidor |

```dart
class ApiException implements Exception {
  final String code;
  final String message;
  final Map<String, String>? fields;

  ApiException({required this.code, required this.message, this.fields});

  factory ApiException.fromDioError(DioException e) {
    final data = e.response?.data;
    if (data is Map && data['error'] != null) {
      final error = data['error'];
      return ApiException(
        code: error['code'] ?? 'UNKNOWN',
        message: error['message'] ?? e.message ?? 'Error desconocido',
        fields: (error['fields'] as Map?)?.cast<String, String>(),
      );
    }
    return ApiException(code: 'UNKNOWN', message: e.message ?? 'Error de red');
  }
}
```
**Importante**: un `404` en `/files/:id` o `/folders/:id` significa "no existe o no es tuyo" — no asumas que el recurso nunca existió, podría ser de otro usuario.

## 7. Límites a tener en cuenta

- **Rate limit**: 30 requests/segundo por IP (configurable en el backend). Pasado el límite: `429 TOO_MANY_REQUEST`.
- **Sin websockets/tiempo real**: todo es request/response. Si necesitás enterarte de cambios entre dispositivos del mismo usuario, hay que hacer polling manual.
- **Upload síncrono**: `POST /files/upload` sube el archivo completo y de ahí a MinIO — para archivos grandes, la request tarda proporcionalmente al tamaño y al ancho de banda del dispositivo. El servidor tolera hasta 60s por request (fotos de 40-50MP de celulares modernos entran cómodas); pasado ese tiempo, la conexión se corta.
- **`POST /files/upload` es de a un archivo por request** — no hay endpoint de batch.

### Subir varios archivos a la vez (evitando el 429)

Disparar todas las subidas en paralelo sin control puede superar el rate limit. Usá el paquete `pool` para acotar la concurrencia (equivalente a `p-limit` en JS):

```dart
Future<void> uploadGallery(List<File> photos, FileService fileService, {String? folderId}) async {
  final pool = Pool(4); // máximo 4 subidas simultáneas
  var completed = 0;

  await Future.wait(photos.map((photo) => pool.withResource(() async {
        await fileService.upload(
          photo,
          folderId: folderId,
          onProgress: (sent, total) {
            // progreso individual de este archivo: sent / total
          },
        );
        completed++;
        // progreso agregado: completed / photos.length
      })));
}
```
Si igual llega un `429` en algún archivo puntual, esperá un instante corto (ej. 500ms) y reintentá **ese** archivo, no toda la selección.

## 8. Flujo típico end-to-end

```
1. POST /user/register                        → crear cuenta
2. POST /user/login                            → guardar token + refreshToken (flutter_secure_storage)
3. POST /folders {"name":"Viajes"}             → crear una carpeta (opcional)
4. POST /files/upload (folderId=…)             → subir una foto
5. GET  /files/list?folderId=…                 → listar lo subido
6. GET  /files/:id/thumbnail-url?size=small    → miniatura para la grilla de la galería
6b. GET /files/:id/thumbnail-url?size=medium   → vista ampliada en el photo viewer
6c. GET /files/:id/url                         → el original (solo al descargar una foto puntual)
7. DELETE /files/:id                           → borrar si hace falta
8. POST /user/refresh {"refreshToken":…}       → el interceptor de Dio lo maneja automáticamente ante un 401
```

## 9. Qué NO existe todavía (no asumir)

- Recuperación de contraseña / cambio de email.
- Compartir archivos o carpetas entre usuarios (`isPublic` existe en el modelo pero no tiene ningún efecto real hoy).
- Metadata real de imagen (`width`/`height`) — las miniaturas ya existen, pero esos dos campos siguen sin calcularse.
- Búsqueda de archivos por nombre.
- Logout / revocación manual de un refresh token del lado del servidor.
- Push notifications / cualquier mecanismo de tiempo real.
